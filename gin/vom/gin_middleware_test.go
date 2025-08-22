package vom

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/constraint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// 1. Define the dynamic view object using the dvo API with validation rules.
var orderVO = dvo.WithFields(
	dvo.Field[string]("OrderID")(),                            // From JSON body
	dvo.Field[string]("CustomerID")(),                         // From JSON body
	dvo.Field[time.Time]("OrderDate")(),                       // From JSON body
	dvo.Field[float64]("Amount", constraint.Gt[float64](0))(), // From JSON body
	dvo.Field[int]("Priority")().Optional(),                   // From JSON body
	dvo.Field[bool]("Shipped")(),                              // From JSON body
	dvo.Field[string]("ordId")().Optional(),                   // From path parameter
	dvo.Field[string]("source")().Optional(),                  // From query parameter
	dvo.Field[int]("limit")().Optional(),                      // From query parameter
	dvo.Field[time.Time]("registered_date")().Optional(),      // From query parameter
	dvo.Field[bool]("received")().Optional(),                  // From query parameter
	dvo.Field[float64]("minim_price")().Optional(),            // From query parameter
)

type MiddlewareTestSuite struct {
	suite.Suite
	srv *gin.Engine
}

func (suite *MiddlewareTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	suite.srv = gin.Default()
	// Set a global enricher for the entire test suite
	SetGlobalEnricher(func(c *gin.Context) map[string]any {
		return map[string]any{
			"traceId": "test-trace-id",
		}
	})
}

func (suite *MiddlewareTestSuite) TearDownSuite() {
	// Reset sync.Once to clean up the global state for other test suites.
	once = sync.Once{}
	// Clean up global enricher after all tests in the suite are done
	SetGlobalEnricher(nil)
}

func TestMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuite))
}

// orderHandler retrieves the validated view object from the context and returns it.
func orderHandler(c *gin.Context) {
	vo := ValueObject(c)
	if vo == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Validated view object not found in context"})
		return
	}
	c.JSON(http.StatusOK, vo)
}

func (suite *MiddlewareTestSuite) TestDynamicVOBinding() {
	// 2. Bind the dynamic ViewObject to the endpoint using the middleware.
	suite.srv.POST("/neworder", Bind(orderVO), orderHandler)
	testCases := []struct {
		name           string
		inputFile      string
		expectedStatus int
	}{
		{
			name:           "Valid Order",
			inputFile:      "testdata/valid_order.json",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Amount (Negative)",
			inputFile:      "testdata/invalid_amount.json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing Required Field (CustomerID)",
			inputFile:      "testdata/missing_customer.json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "optional (Priority)",
			inputFile:      "testdata/valid_order_optional.json",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "optional (Priority) invalid type",
			inputFile:      "testdata/invalid_order_optional.json",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// 3. Issue a request using data from the specified file.
			payloadBytes, err := os.ReadFile(tc.inputFile)
			require.NoError(suite.T(), err)
			payload := string(payloadBytes)

			req := httptest.NewRequest(http.MethodPost, "/neworder", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			suite.srv.ServeHTTP(rec, req)
			// 4. validate the outcome.
			assert.Equal(suite.T(), tc.expectedStatus, rec.Code)
			// For the successful case, also verify the content of the response.
			if tc.expectedStatus == http.StatusOK {
				// The handler returns the validated object, which includes the globally enriched 'traceId'.
				// To create the expected JSON, we unmarshal the original payload, add the traceId,
				// and then marshal it back. This avoids adding new dependencies.
				var expectedMap map[string]any
				err := json.Unmarshal(payloadBytes, &expectedMap)
				require.NoError(suite.T(), err)

				expectedMap["traceId"] = "test-trace-id"

				expectedPayloadBytes, err := json.Marshal(expectedMap)
				require.NoError(suite.T(), err)
				assert.JSONEq(suite.T(), string(expectedPayloadBytes), rec.Body.String())
			}
		})
	}
}

func (suite *MiddlewareTestSuite) TestBindingWithParamsAndEnricher() {
	// Set up the route with the binding middleware.
	// We use a unique route to avoid conflicts with other tests in the suite.
	suite.srv.POST("/enriched_orders/:ordId", Bind(orderVO), orderHandler)

	testCases := []struct {
		name           string
		url            string
		inputFile      string
		expectedStatus int
		expectedValues map[string]any // Expected values from URL params to merge into the final JSON
		shouldPanic    bool
	}{
		{
			name:           "Valid request with basic path and query parameters",
			url:            "/enriched_orders/order-abc-123?source=web",
			inputFile:      "testdata/valid_order_optional.json",
			expectedStatus: http.StatusOK,
			expectedValues: map[string]any{"ordId": "order-abc-123", "source": "web"},
		},
		{
			name:           "Valid request with all optional query parameters",
			url:            "/enriched_orders/order-xyz-789?source=api&limit=100&registered_date=2024-01-15T10:00:00Z&received=true&minim_price=99.99",
			inputFile:      "testdata/valid_order.json",
			expectedStatus: http.StatusOK,
			expectedValues: map[string]any{
				"ordId":           "order-xyz-789",
				"source":          "api",
				"limit":           100,
				"registered_date": "2024-01-15T10:00:00Z",
				"received":        true,
				"minim_price":     99.99,
			},
		},
		{
			name:        "Request with multiple values for a query parameter should panic",
			url:         "/enriched_orders/order-fail-case?source=web&source=api",
			inputFile:   "testdata/valid_order_optional.json",
			shouldPanic: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			payloadBytes, err := os.ReadFile(tc.inputFile)
			require.NoError(suite.T(), err)

			req := httptest.NewRequest(http.MethodPost, tc.url, strings.NewReader(string(payloadBytes)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			if tc.shouldPanic {
				// The default Gin recovery middleware will catch the panic and return a 500 error.
				// So, we assert the status code instead of the panic itself.
				suite.srv.ServeHTTP(rec, req)
				assert.Equal(suite.T(), http.StatusInternalServerError, rec.Code)
				return
			}

			suite.srv.ServeHTTP(rec, req)
			require.Equalf(suite.T(), tc.expectedStatus, rec.Code, rec.Body.String())

			var expectedMap map[string]any
			err = json.Unmarshal(payloadBytes, &expectedMap)
			require.NoError(suite.T(), err)

			// Merge URL params and enricher data to build the final expected JSON
			for k, v := range tc.expectedValues {
				expectedMap[k] = v
			}
			expectedMap["traceId"] = "test-trace-id"

			expectedPayloadBytes, err := json.Marshal(expectedMap)
			require.NoError(suite.T(), err)
			assert.JSONEq(suite.T(), string(expectedPayloadBytes), rec.Body.String())
		})
	}
}
