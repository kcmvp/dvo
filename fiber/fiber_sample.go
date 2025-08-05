////go:build ignore

package main

import (
	"github.com/gofiber/fiber/v3"
	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/fiber/middleware"
	"github.com/kcmvp/dvo/validator"
	"log"
)

// In a real-world application, the ViewObject definitions below would typically
// reside in a dedicated package, such as `vo` or `dto`.
// For example: `vo.Login`. This promotes separation of concerns and reusability.
var loginVO = dvo.WithFields(
	// dvo.Field returns a FieldFunc, which must be called to produce the field.
	dvo.Field[string]("username", validator.Email())(),
	dvo.Field[string]("password", validator.MinLength(8))(),
)

// loginHandler is the main business logic handler.
// It runs only after the Bind middleware has successfully validated the request.
func loginHandler(c fiber.Ctx) error {
	// Use the framework-specific helper from the adaptor package to get the ViewObject.
	vo := middleware.ViewObject(c)
	if vo == nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Validated view object not found in context")
	}

	// The object is returned directly as the JSON response.
	// This demonstrates the generic nature of the ViewObject, which is a
	// validated container for the request data.
	return c.JSON(vo)
}

func main() {
	// 1. Set up the Fiber app
	app := fiber.New()

	// 2. Bind the ViewObject blueprint to the route.
	app.Post("/login", middleware.Bind(loginVO), loginHandler)

	// 3. Start the server and provide test commands.
	port := "8080"
	log.Printf("Fiber server running on port %s", port)
	log.Println("---")
	log.Println("Test with a valid request:")
	log.Printf(`curl -X POST -H "Content-Type: application/json" -d ''''{"username":"test@example.com", "password":"a-long-password"}'''' http://localhost:%s/login`, port)

	log.Fatal(app.Listen(":" + port))
}
