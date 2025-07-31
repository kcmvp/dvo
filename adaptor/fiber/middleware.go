package fiber

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/kcmvp/dvo"
)

// DVOMiddleware provides a Fiber-compatible middleware function that wraps the
// core dvo.Middleware.
//
// It leverages Fiber's built-in adapter to seamlessly convert the standard
// net/http middleware, providing a consistent and discoverable API for
// developers using the Fiber framework.
func DVOMiddleware(voName string, opts ...dvo.MiddlewareOption) fiber.Handler {
	return adaptor.HTTPMiddleware(dvo.Middleware(voName, opts...))
}
