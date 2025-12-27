package vom

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/kcmvp/dvo/internal"
	"github.com/kcmvp/dvo/view"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"github.com/samber/mo"
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

func urlParams(ctx echo.Context) map[string]string {
	params := lo.Associate(ctx.ParamNames(), func(name string) (string, string) {
		return name, ctx.Param(name)
	})
	// Add query parameters
	for name, values := range ctx.QueryParams() {
		lo.Assertf(len(values) == 1, "query parameter '%s' has multiple values, which is not supported", name)
		_, ok := params[name]
		lo.Assertf(!ok, "path parameter and query parameter have conflicting names: '%s'", name)
		params[name] = values[0]
	}
	return params

}

// Bind returns an Echo middleware function that validates incoming JSON request bodies
// against the provided dvo.Schema schema.
func Bind(schema *view.Schema) echo.MiddlewareFunc {
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
			// validate the JSON body against the Schema schema.
			result := schema.Validate(body, urlParams(c))
			if result.IsError() {
				// If validation fails, return a 400 Bad Request with a structured error message
				// containing the details of the validation failures.
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": result.Error().Error(),
				})
			}
			data := result.MustGet()

			// If an enrichment function has been provided via SetGlobalEnricher, apply it to add
			// or overwrite data in the ValueObject. This is useful for adding data from
			// path parameters, headers, or other request context.
			if _enrich != nil {
				for k, v := range _enrich(c) {
					data.Add(k, v)
				}
			}

			// Store the validated and enriched ValueObject in the request's context
			// for downstream handlers to access using the dvo.ViewObjectKey.
			req := c.Request()
			ctx := context.WithValue(req.Context(), internal.ViewObjectKey, data)
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
func ValueObject(c echo.Context) view.ValueObject {
	// Retrieve the value from the request's context using the predefined key.
	if val := c.Request().Context().Value(internal.ViewObjectKey); val != nil {
		// Perform a type assertion to ensure the value is of the expected type.
		if vo, ok := val.(view.ValueObject); ok {
			return vo
		}
	}
	// Return nil if the value is not found or the type assertion fails.
	return nil
}
