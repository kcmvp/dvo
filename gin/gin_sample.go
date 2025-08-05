////go:build ignore

package main

import (
	"github.com/gin-gonic/gin"
	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/gin/middelware"
	"github.com/kcmvp/dvo/validator"
	"log"
	"net/http"
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
func loginHandler(c *gin.Context) {
	// Use the framework-specific helper from the adaptor package to get the ViewObject.
	vo := middelware.ViewObject(c)
	if vo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Validated view object not found in context"})
		return
	}

	// The object is returned directly as the JSON response.
	// This demonstrates the generic nature of the ViewObject, which is a
	// validated container for the request data.
	c.JSON(http.StatusOK, vo)
}

func main() {
	// 1. Set up the Gin router
	router := gin.Default()

	// 2. Bind the ViewObject blueprint to the route.
	router.POST("/login", middelware.Bind(loginVO), loginHandler)

	// 3. Start the server and provide test commands.
	port := "8080"
	log.Printf("Gin server running on port %s", port)
	log.Println("---")

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
