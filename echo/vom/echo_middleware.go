package vom

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/kcmvp/dvo"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/tidwall/gjson"
)

// EnrichFunc defines a function type that can enrich the validated data with additional key-value pairs.
// It takes an `echo.Context` as input and returns a map of string to any, representing the data to be added.
type EnrichFunc func(echo.Context) map[string]any

// _enrich is a private variable of type `EnrichFunc` that stores the enrichment function provided by the user.
// It is initialized once using `sync.Once`.
var _enrich EnrichFunc

// once is a `sync.Once` variable used to ensure that the `_enrich` function is set only once.
// This prevents multiple calls to `SetGlobalEnricher` from overwriting the enrichment function.
var once sync.Once

// SetGlobalEnricher sets the enrichment function to be called upon successful validation of the request body.
// This function should be called only once during application startup.
// Subsequent calls will be ignored.
func SetGlobalEnricher(enrich EnrichFunc) {
	once.Do(func() {
		_enrich = enrich
	})
}

// Bind returns an Echo middleware function that validates incoming JSON request bodies
// against the provided dvo.ViewObject schema.
func Bind(vo *dvo.ViewObject) echo.MiddlewareFunc {
	// The returned function is the actual middleware that will be executed for each request.
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		// This is the handler function that Echo will call.
		return func(c echo.Context) error {
			// Read the entire request body. The body is read into memory here.
			// For very large request bodies, a streaming approach might be preferable.
			btsResult := mo.TupleToResult(io.ReadAll(c.Request().Body))
			if btsResult.IsError() {
				// If reading the body fails, return a Bad Request response.
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
			}
			body := string(btsResult.MustGet())

			// Ensure the request body is valid JSON before proceeding with schema validation.
			if !gjson.Valid(body) {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON format"})
			}

			// Validate the JSON body against the ViewObject schema.
			result := vo.Validate(body)
			if result.IsError() {
				// If validation fails, return a 400 Bad Request with a structured error message
				// containing the details of the validation failures.
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": result.Error().Error(),
				})
			}

			// On successful validation, retrieve the resulting ValueObject.
			data := result.MustGet()

			// If an enrichment function has been provided via SetGlobalEnricher, apply it to add
			// or overwrite data in the ValueObject. This is useful for adding data from
			// path parameters, headers, or other request context.
			if _enrich != nil {
				for k, v := range _enrich(c) {
					op := data.Get(k)
					lo.Assertf(op.IsPresent(), "property %s exiests", k)
					data.Set(k, v)
				}
			}

			// Store the validated and enriched ValueObject in the request's context
			// for downstream handlers to access using the dvo.ViewObjectKey.
			req := c.Request()
			ctx := context.WithValue(req.Context(), dvo.ViewObjectKey, data)
			c.SetRequest(req.WithContext(ctx))

			// Proceed to the next middleware or the main handler in the chain.
			return next(c)
		}
	}
}

// ValueObject retrieves the validated dvo.ValueObject from the echo context.
// This helper function should be used within your route handlers to access the
// type-safe data that has been processed by the Bind middleware.
// It returns nil if the ValueObject is not found in the context, which might
// happen if the middleware was not used for the route.
func ValueObject(c echo.Context) dvo.ValueObject {
	// Retrieve the value from the request's context using the predefined key.
	if val := c.Request().Context().Value(dvo.ViewObjectKey); val != nil {
		// Perform a type assertion to ensure the value is of the expected type.
		if vo, ok := val.(dvo.ValueObject); ok {
			return vo
		}
	}
	// Return nil if the value is not found or the type assertion fails.
	return nil
}
