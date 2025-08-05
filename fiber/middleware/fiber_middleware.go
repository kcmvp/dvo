package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/kcmvp/dvo"
	"github.com/tidwall/gjson"
	"net/http"
	"sync"
)

// Enrich is a function type that can be used to enrich the validated data
// with additional information from the Fiber context.
// It takes a Fiber context as input and returns a map of string to any,
// which will be merged into the validated data. This function is executed only once.
type Enrich func(c fiber.Ctx) map[string]any

var _enrich Enrich
var once sync.Once

// EnrichViewWith sets a function to enrich the validated data.
// This function is executed only once, typically during application startup.
// The provided `enrich` function will be called after successful validation
// to add additional information to the data object.
func EnrichViewWith(enrich Enrich) {
	once.Do(func() {
		_enrich = enrich
	})
}

// Bind creates a new fiber middleware to bind and validate the view object.
// It takes a provider function that returns a new ViewObject for each request.
func Bind(vo *dvo.ViewObject) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get a fresh ViewObject instance for this request.
		body := string(c.Body())
		if !gjson.Valid(body) {
			return c.JSON(http.StatusBadRequest, "Invalid JSON")
		}
		// The Validate method is defined in the internal/core package.
		result := vo.Validate(body)

		// The Validate method is defined in the internal/core package.
		if result.IsError() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": result.Error().Error()})
		}
		data := result.MustGet()
		if _enrich != nil {
			for k, v := range _enrich(c) {
				data.Set(k, v)
			}
		}
		// Store the validated object in the context for the main handler to use.
		c.Locals(dvo.ViewObjectKey, data)
		return c.Next()
	}
}

// ViewObject retrieves the validated ViewObject from the fiber context.
// It returns nil if the object is not found.
func ViewObject(c fiber.Ctx) dvo.DataObject {
	if val := c.Locals(dvo.ViewObjectKey); val != nil {
		if vo, ok := val.(dvo.DataObject); ok {
			return vo
		}
	}
	return nil
}
