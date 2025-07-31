package gin

import (
	"github.com/gin-gonic/gin"
	"github.com/kcmvp/dvo"
	"net/http"
)

// DVOMiddleware wraps the core dvo middleware for use with the Gin framework.
// It accepts the same options as the core middleware.
func DVOMiddleware(voName string, opts ...dvo.MiddlewareOption) gin.HandlerFunc {
	// Create the core net/http middleware constructor.
	coreMiddlewareConstructor := dvo.Middleware(voName, opts...)

	return func(c *gin.Context) {
		// The handler that Gin would call next.
		// We wrap it in an http.HandlerFunc so the core middleware can call it.
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The core middleware might have replaced the request context.
			// We need to make sure Gin's context uses the new one.
			c.Request = r
			c.Next()
		})

		// We apply the core middleware to our 'next' handler,
		// which returns a new http.Handler that we can serve.
		handlerToExecute := coreMiddlewareConstructor(nextHandler)
		handlerToExecute.ServeHTTP(c.Writer, c.Request)

		// If the middleware wrote an error and stopped, we should abort the Gin chain.
		if c.IsAborted() {
			return
		}
	}
}
