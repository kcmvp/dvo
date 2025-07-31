package echo

import (
	"github.com/kcmvp/dvo"
	"github.com/labstack/echo/v4"
)

// DVOMiddleware creates an Echo middleware that validates and binds incoming request data
// to a specified Value Object (VO). It leverages the dvo.Middleware for the core
// validation logic and then wraps it for compatibility with Echo's middleware system.
func DVOMiddleware(voName string, opts ...dvo.MiddlewareOption) echo.MiddlewareFunc {
	// Create the core net/http middleware constructor.
	coreMiddlewareConstructor := dvo.Middleware(voName, opts...)

	// Use Echo's built-in function to wrap it.
	return echo.WrapMiddleware(coreMiddlewareConstructor)
}
