package middelware

import (
	"context"
	"github.com/kcmvp/dvo"
	"github.com/labstack/echo/v4"
	"github.com/samber/mo"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"sync"
)

// Enrich defines a function type that can enrich the validated data with additional key-value pairs.
// It takes an `echo.Context` as input and returns a map of string to any, representing the data to be added.
type Enrich func(echo.Context) map[string]any

// _enrich is a private variable of type `Enrich` that stores the enrichment function provided by the user.
// It is initialized once using `sync.Once`.
var _enrich Enrich

// once is a `sync.Once` variable used to ensure that the `_enrich` function is set only once.
// This prevents multiple calls to `OnSuccessfulValidation` from overwriting the enrichment function.
var once sync.Once

// OnSuccessfulValidation sets the enrichment function to be called upon successful validation of the request body.
// This function should be called only once during application startup.
// Subsequent calls will be ignored.
func OnSuccessfulValidation(enrich Enrich) {
	once.Do(func() {
		_enrich = enrich
	})
}

func Bind(vo *dvo.ViewObject) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			bts := mo.TupleToResult[[]byte](io.ReadAll(c.Request().Body))
			if bts.IsError() {
				return c.JSON(http.StatusBadRequest, bts.Error().Error())
			}
			body := string(bts.MustGet())
			if !gjson.Valid(body) {
				return c.JSON(http.StatusBadRequest, "Invalid JSON")
			}
			// The Validate method is defined in the internal/core package.
			result := vo.Validate(body)
			if result.IsError() {
				// If validation fails, return a 400 Bad Request with a structured error.
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": result.Error().Error(),
				})
			}

			data := result.MustGet()
			if _enrich != nil {
				for k, v := range _enrich(c) {
					data[k] = v
				}
			}
			// On success, store the validated clone in the standard request context.
			req := c.Request()
			ctx := context.WithValue(req.Context(), dvo.ViewObjectKey, result.MustGet())
			c.SetRequest(req.WithContext(ctx))

			// Proceed to the next middleware or the main handler.
			return next(c)
		}
	}
}

// ViewObject retrieves the validated ViewObject from the echo context.
// It returns nil if the object is not found.
func ViewObject(c echo.Context) dvo.Data {
	if val := c.Request().Context().Value(dvo.ViewObjectKey); val != nil {
		if vo, ok := val.(dvo.Data); ok {
			return vo
		}
	}
	return nil
}
