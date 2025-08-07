////go:build ignore

package main

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/constraint"
	"github.com/kcmvp/dvo/fiber/middleware"
)

// In a real-world application, the ValueObject definitions below would typically
// reside in a dedicated package, such as `vo` or `dto`.
// For example: `vo.Login`. This promotes separation of concerns and reusability.
var loginVO = dvo.WithFields(
	// dvo.Field returns a FF, which must be called to produce the field.
	dvo.Field[string]("username", constraint.Email())(),
	dvo.Field[string]("password", constraint.MinLength(8))(),
)

// loginHandler is the main business logic handler.
// It runs only after the Bind middleware has successfully validated the request.
func loginHandler(c fiber.Ctx) error {
	// Use the framework-specific helper from the adaptor package to get the ValueObject.
	vo := middleware.ValueObject(c)
	if vo == nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Validated view object not found in context")
	}

	// The object is returned directly as the JSON response.
	// This demonstrates the generic nature of the ValueObject, which is a
	// validated container for the request data.
	return c.JSON(vo)
}

func main() {
	// 1. Set up the Fiber app
	app := fiber.New()

	// 2. Bind the ValueObject blueprint to the route.
	app.Post("/login", middleware.Bind(loginVO), loginHandler)

	// 3. Start the server and provide test commands.
	port := "8080"
	log.Printf("Fiber server running on port %s", port)
	log.Println("---")
	log.Println("Test with a valid request:")
	log.Printf(`curl -X POST -H "Content-Type: application/json" -d ''''{"username":"test@example.com", "password":"a-long-password"}'''' http://localhost:%s/login`, port)

	log.Fatal(app.Listen(":" + port))
}
