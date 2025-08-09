<p align="center">
  Declarative View Object (DVO)
  <br/>
  <br/>
  <a href="https://github.com/kcmvp/dvo/blob/main/LICENSE">
    <img alt="GitHub" src="https://img.shields.io/github/license/kcmvp/dvo"/>
  </a>
  <a href="https://goreportcard.com/report/github.com/kcmvp/dvo">
    <img src="https://goreportcard.com/badge/github.com/kcmvp/dvo"/>
  </a>
  <a href="https://pkg.go.dev/github.com/kcmvp/dvo">
    <img src="https://pkg.go.dev/badge/github.com/kcmvp/dvo.svg" alt="Go Reference"/>
  </a>
  <a href="https://github.com/kcmvp/archunit/blob/main/.github/workflows/build.yml" rel="nofollow">
     <img src="https://img.shields.io/github/actions/workflow/status/kcmvp/dvo/build.yml?branch=main" alt="Build" />
  </a>
  <a href="https://app.codecov.io/gh/kcmvp/dvo" ref="nofollow">
    <img src ="https://img.shields.io/codecov/c/github/kcmvp/dvo" alt="coverage"/>
  </a>

</p>

**dvo** is a powerful, source-agnostic validation framework for Go that eliminates redundant and scattered validation logic.

We've all felt the pain. For one endpoint, you're meticulously adding struct tags to a JSON request body. For another, you're writing a tangled web of `if/else` statements to validate URL query parameters. The logic for validating the same piece of data, like a user ID, ends up duplicated in multiple places. When a rule changes, you have to hunt down every instance, hoping you don't miss one. This is brittle, error-prone, and doesn't scale.

**dvo** solves this by creating a single source of truth. It provides a fluent, declarative API to build reusable validation schemas (`ValueObject`) that are completely decoupled from the data's origin. You define the validation rules for a concept like 'username' or 'paging' once, and then apply that schema to request bodies, query parameters, or any other data source. This is the core principle of **dvo**: centralize your validation logic, simplify maintenance, and build dramatically more robust and reliable APIs.

## Features

- **Declarative API:** Define validation schemas for your request data in a clear, readable, and reusable way.
- **Framework Middleware:** Out-of-the-box integration with Gin, Echo, and Fiber.
- **Extensible Enrichment:** A powerful "Global Enricher" pattern to inject common data (e.g., user info from an auth middleware) into your validated objects automatically.
- **Type-Safe Access:** The validation layer ensures data types are correct before your handler logic runs.
- **Common Constraints:** Includes a set of common validation constraints like `Gt`, `MinLength`, `Pattern`, and more.

## Installation

```bash
go get github.com/kcmvp/dvo
```

## Core Concepts

The library revolves around the `ValueObject`, which acts as a reusable validation schema for your request data. You define the schema by composing `Field` definitions, each with its own type and validation constraints.

A `ValueObject` is typically defined once at the package level and reused across different handlers.

```go
import (
    "time"
    "github.com/kcmvp/dvo"
    "github.com/kcmvp/dvo/constraint"
)

// Define a validation schema for a new order.
// This schema is reusable and can be passed to the validation middleware.
var orderVO = dvo.WithFields(
    dvo.Field[string]("OrderID")(),
    dvo.Field[string]("CustomerID")(),
    dvo.Field[time.Time]("OrderDate")(),
    dvo.Field[float64]("Amount", constraint.Gt[float64](0))(),
    dvo.Field[int]("Priority")().Optional(), // This field is not required
    dvo.Field[bool]("Shipped")(),
)
```

## Usage with Web Frameworks

`dvo` provides middleware for popular frameworks to make data binding and validation a single, clean step. If validation fails, the middleware will automatically abort the request and send a `400 Bad Request` response.

### Gin

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/kcmvp/dvo/gin/vom" // Gin Validation Middleware
)

// 1. Define your handler to access the validated data.
func orderHandler(c *gin.Context) {
    // The middleware places the validated data into the context.
    vo := vom.ValueObject(c)
    // Your logic here...
    c.JSON(http.StatusOK, vo)
}

// 2. Set up your router.
func setupRouter() *gin.Engine {
    router := gin.Default()
    // 3. Apply the Bind middleware with your validation schema.
    router.POST("/neworder", vom.Bind(orderVO), orderHandler)
    return router
}
```

### Echo

```go
import (
    "github.com/labstack/echo/v4"
    "github.com/kcmvp/dvo/echo/vom" // Echo Validation Middleware
)

// 1. Define your handler.
func orderHandler(c echo.Context) error {
    vo := vom.ValueObject(c)
    // Your logic here...
    return c.JSON(http.StatusOK, vo)
}

// 2. Set up your router.
func setupRouter() *echo.Echo {
    e := echo.New()
    // 3. Apply the Bind middleware.
    e.POST("/neworder", vom.Bind(orderVO)(orderHandler))
    return e
}
```

### Fiber

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/kcmvp/dvo/fiber/vom" // Fiber Validation Middleware
)

// 1. Define your handler.
func orderHandler(c *fiber.Ctx) error {
    vo := vom.ValueObject(c)
    // Your logic here...
    return c.Status(fiber.StatusOK).JSON(vo)
}

// 2. Set up your router.
func setupRouter() *fiber.App {
    app := fiber.New()
    // 3. Apply the Bind middleware.
    app.Post("/neworder", vom.Bind(orderVO), orderHandler)
    return app
}
```

## Global Enricher

You can set a global `Enricher` function that runs after successful validation but before your handler. This is ideal for injecting common data, like a user ID from an authentication middleware. The map returned by the enricher is merged into the validated `ValueObject`.

This function is set **once** at application startup.

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/kcmvp/dvo/gin/vom"
)

// An enricher function for Gin.
func addUser(c *gin.Context) map[string]any {
    // Assumes a previous auth middleware has set the userID.
    userID, _ := c.Get("userID")
    return map[string]any{
        "userID": userID,
    }
}

func main() {
    // Set the enricher once.
    vom.SetGlobalEnricher(addUser)

    // ... setup router and start server
}
```
