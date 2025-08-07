////go:build ignore

package main

import (
	"log"
	"net/http"

	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/constraint"
	"github.com/kcmvp/dvo/echo/middelware"
	"github.com/labstack/echo/v4"
)

// In a real-world application, the ValueObject definitions below would typically
// reside in a dedicated package, such as `vo` or `dto`.
// For example: `vo.Login`. This promotes separation of concerns and reusability.
var loginVO = dvo.WithFields(
	// Field returns a FF, which we call to get the configured field.
	dvo.Field[string]("username", constraint.Email())(),
	dvo.Field[string]("password", constraint.Match("*abc"))(),
	dvo.Field[bool]("rememberMe", constraint.BeTrue())(),
	dvo.Field[string]("testing", constraint.CharSetAny(constraint.LowerCaseChar))(),
).AllowUnknownFields()

// loginHandler is the main business logic handler.
// It runs only after the Bind middleware has successfully validated the request.
func loginHandler(c echo.Context) error {
	// Use the framework-specific helper from the adaptor package to get the ValueObject.
	vo := middelware.ValueObject(c)
	if vo == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Validated view object not found in context"})
	}

	// The object is returned directly as the JSON response.
	// This demonstrates the generic nature of the ValueObject, which is a
	// validated container for the request data.
	return c.JSON(http.StatusOK, vo)
}

func main() {
	// 1. Set up the Echo app
	e := echo.New()

	// 2. Bind the ValueObject blueprint to the route.
	//    Note: In Echo, middleware is applied after the handler in the argument list.
	e.POST("/login", loginHandler, middelware.Bind(loginVO))

	// 3. Start the server and provide test commands.
	port := "8080"
	log.Printf("Echo server running on port %s", port)
	log.Println("Use 'go run echo/echo_sample.go' to start.")

	if err := e.Start(":" + port); err != nil {
		e.Logger.Fatal(err)
	}
}
