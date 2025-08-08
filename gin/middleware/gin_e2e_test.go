package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/constraint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 1. Define the dynamic view object using the dvo API with validation rules.
var orderVO = dvo.WithFields(
	dvo.Field[string]("OrderID")(),
	dvo.Field[string]("CustomerID")(),
	dvo.Field[time.Time]("OrderDate")(),
	dvo.Field[float64]("Amount", constraint.Gt[float64](0))(),
	dvo.Field[int]("Priority")().Optional(),
	dvo.Field[bool]("Shipped")(),
)

func setupRouter() *gin.Engine {
	router := gin.Default()
	router.POST("/neworder", Bind(orderVO), orderHandler)
	return router
}

// orderHandler retrieves the validated view object from the context and returns it.
func orderHandler(c *gin.Context) {
	vo := ValueObject(c)
	if vo == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	c.JSON(http.StatusOK, vo)
}

func TestDynamicVOBinding(t *testing.T) {

	router := setupRouter()

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
		t.Run(tc.name, func(t *testing.T) {
			// 3. Issue a request using data from the specified file.
			payloadBytes, err := os.ReadFile(tc.inputFile)
			require.NoError(t, err)
			payload := string(payloadBytes)

			rec := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/neworder", strings.NewReader(payload))

			router.ServeHTTP(rec, req)
			// 4. Validate the outcome.
			assert.Equal(t, tc.expectedStatus, rec.Code)
			// For the successful case, also verify the content of the response.
			if tc.expectedStatus == http.StatusOK {
				// The handler returns the validated object, so the response should match the input.
				assert.JSONEq(t, payload, rec.Body.String())
			}
		})
	}
}
