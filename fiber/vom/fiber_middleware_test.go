package vom

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
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

// orderHandler retrieves the validated view object from the context and returns it.
func orderHandler(c fiber.Ctx) error {
	vo := ValueObject(c)
	if vo == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "can not find viewObject"})
	}
	return c.JSON(vo)
}

func setupRouter() *fiber.App {
	app := fiber.New()
	app.Post("/neworder", Bind(orderVO), orderHandler)
	return app
}

func TestDynamicVOBinding(t *testing.T) {

	app := setupRouter()

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

			req, _ := http.NewRequest("POST", "/neworder", strings.NewReader(payload))
			// Perform the request using app.Test
			res, err := app.Test(req)
			// 4. validate the outcome.
			assert.Equal(t, tc.expectedStatus, res.StatusCode)
			// For the successful case, also verify the content of the response.
			if tc.expectedStatus == http.StatusOK {
				// The handler returns the validated object, so the response should match the input.
				body, _ := io.ReadAll(res.Body)
				assert.JSONEq(t, payload, string(body))
			}
		})
	}
}

func TestSetGlobalEnricher(t *testing.T) {

	// Define two different enricher functions.
	firstEnricher := func(c fiber.Ctx) map[string]any {
		return map[string]any{
			"id": "1",
		}
	}
	secondEnricher := func(c fiber.Ctx) map[string]any {
		return map[string]any{
			"id": "2",
		}
	}

	// Call SetGlobalEnricher twice. The sync.Once should ignore the second call.
	SetGlobalEnricher(firstEnricher)
	SetGlobalEnricher(secondEnricher)
	v1, _ := firstEnricher(nil)["id"]
	v2, _ := _enrich(nil)["id"]
	assert.True(t, v1 == v2)
}
