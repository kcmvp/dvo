package vom

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/kcmvp/xql/validator"
	"github.com/kcmvp/xql/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// 1. Define the dynamic view object using the dvo API with validation rules.
var orderVO = view.WithFields(
	view.Field[string]("OrderID"),                           // From JSON body
	view.Field[string]("CustomerID"),                        // From JSON body
	view.Field[time.Time]("OrderDate"),                      // From JSON body
	view.Field[float64]("Amount", validator.Gt[float64](0)), // From JSON body
	view.Field[int]("Priority").Optional(),                  // From JSON body
	view.Field[bool]("Shipped"),                             // From JSON body
	view.Field[string]("ordId").Optional(),                  // From path parameter
	view.Field[string]("source").Optional(),                 // From query parameter
	view.Field[int]("limit").Optional(),                     // From query parameter
	view.Field[time.Time]("registered_date").Optional(),     // From query parameter
	view.Field[bool]("received").Optional(),                 // From query parameter
	view.Field[float64]("minim_price").Optional(),           // From query parameter
)

type MiddlewareTestSuite struct {
	suite.Suite
	srv *fiber.App
}

func (suite *MiddlewareTestSuite) SetupSuite() {
	suite.srv = fiber.New()
	// Set a global enricher for the entire test suite
	SetGlobalEnricher(func(c fiber.Ctx) map[string]any {
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
func orderHandler(c fiber.Ctx) error {
	vo := ValueObject(c)
	if vo == nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Validated view object not found in context"})
	}
	return c.JSON(vo)
}

func (suite *MiddlewareTestSuite) TestDynamicVOBinding() {
	// 2. Bind the dynamic ViewObject to the endpoint using the middleware.
	suite.srv.Post("/neworder", Bind(orderVO), orderHandler)
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

			req, _ := http.NewRequest(http.MethodPost, "/neworder", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")

			res, err := suite.srv.Test(req)
			require.NoError(suite.T(), err)

			// 4. validate the outcome.
			assert.Equal(suite.T(), tc.expectedStatus, res.StatusCode)
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
				body, _ := io.ReadAll(res.Body)
				assert.JSONEq(suite.T(), string(expectedPayloadBytes), string(body))
			}
		})
	}
}

func (suite *MiddlewareTestSuite) TestBindingWithParamsAndEnricher() {
	// Set up the route with the binding middleware.
	// We use a unique route to avoid conflicts with other tests in the suite.
	suite.srv.Post("/enriched_orders/:ordId", Bind(orderVO), orderHandler)

	testCases := []struct {
		name           string
		url            string
		inputFile      string
		expectedStatus int
		expectedValues map[string]any // Expected values from URL params to merge into the final JSON
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
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			payloadBytes, err := os.ReadFile(tc.inputFile)
			require.NoError(suite.T(), err)

			req, _ := http.NewRequest(http.MethodPost, tc.url, strings.NewReader(string(payloadBytes)))
			req.Header.Set("Content-Type", "application/json")

			res, err := suite.srv.Test(req)
			require.NoError(suite.T(), err)
			bodyBytes, _ := io.ReadAll(res.Body)
			require.Equalf(suite.T(), tc.expectedStatus, res.StatusCode, "Response body: %s", string(bodyBytes))

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
			assert.JSONEq(suite.T(), string(expectedPayloadBytes), string(bodyBytes))
		})
	}
}

// TestPanicOnDuplicateQueryParam tests the specific case where the middleware should panic.
// It is a separate test function to ensure complete isolation from the test suite's shared app instance,
// which can cause routing issues when using the net/http adaptor.
func TestPanicOnDuplicateQueryParam(t *testing.T) {
	// 1. Define the ViewObject needed for this test
	vo := view.WithFields(
		view.Field[string]("ordId"),
		view.Field[string]("source"),
	)

	// 2. Create a new, isolated Fiber app and register the route
	// We disable the default recovery middleware so that we can test for the panic itself.
	// We provide a custom ErrorHandler that will re-panic, allowing assert.Panics to catch it.
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			panic(err)
		},
	})
	app.Post("/enriched_orders/:ordId", Bind(vo), orderHandler)

	// 3. Create the request that should trigger the panic
	url := "/enriched_orders/order-fail-case?source=web&source=api"
	req := httptest.NewRequest(http.MethodPost, url, nil)
	req.Header.Set("Content-Type", "application/json")

	// 4. Use the adaptor to make the call synchronous. Since recovery is disabled,
	// the panic will propagate and can be caught by assert.Panics.
	handler := adaptor.FiberApp(app)
	rec := httptest.NewRecorder()
	assert.Panics(t, func() {
		handler.ServeHTTP(rec, req)
	})
}
