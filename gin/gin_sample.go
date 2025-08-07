////go:build ignore

package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/constraint"
	"github.com/kcmvp/dvo/gin/middelware"
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
func loginHandler(c *gin.Context) {
	// Use the framework-specific helper from the adaptor package to get the ValueObject.
	vo := middelware.ValueObject(c)
	if vo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Validated view object not found in context"})
		return
	}

	// The object is returned directly as the JSON response.
	// This demonstrates the generic nature of the ValueObject, which is a
	// validated container for the request data.
	c.JSON(http.StatusOK, vo)
}

func main() {
	// 1. Set up the Gin router
	router := gin.Default()

	// 2. Bind the ValueObject blueprint to the route.
	router.POST("/login", middelware.Bind(loginVO), loginHandler)

	// 3. Start the server and provide test commands.
	port := "8080"
	log.Printf("Gin server running on port %s", port)
	log.Println("---")

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
