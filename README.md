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

`dvo` is a Go library designed to simplify and streamline request body validation and data binding in modern web frameworks. It provides a declarative, fluent API for defining validation rules, keeping your handler logic clean and focused on business concerns.

## Features

- **Declarative API:** Define validation rules for your data structures in a clear, readable, and chainable way.
- **Framework Adaptors:** Out-of-the-box integration with popular web frameworks like Gin.
- **Extensible Enrichment:** A powerful "Global Enricher" pattern to inject common data (e.g., user info) into your validated objects automatically.
- **Type-Safe Access:** Easily access validated data from the request context.
- **Common Validations:** Includes a set of common validators like `Required`, `Min`, `Max`, `Pattern`, and more.

## Installation

```bash
go get github.com/kcmvp/dvo
```

## Core Concepts

The library revolves around the `ViewObject`, which acts as a blueprint for your request data. You define `Fields` on this object and chain validation rules to them.

```go
import "github.com/kcmvp/dvo"

// A provider function ensures a fresh ViewObject is used for each request.
var signupVOProvider = func() *dvo.ViewObject {
    return dvo.New(
        dvo.WithFields(
            dvo.Field[string]("username").Required().Min(5),
            dvo.Field[string]("password").Required().Min(8),
            dvo.Field[string]("email").Required().Pattern(dvo.Email),
        ),
    )
}
```

## Usage with Gin

`dvo` makes Gin handler logic incredibly clean by removing validation boilerplate.

### Step 1: Define an (Optional) Global Enricher

An `Enricher` is a function that runs immediately after successful validation, allowing you to add common data to the validated object. This is perfect for injecting user information from an auth middleware.

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/kcmvp/dvo"
    "github.com/kcmvp/dvo/gin/adaptor"
)

// 1. Define your enricher function.
// This function will be our global "hook".
func AddUserInfo(c *gin.Context, vo *dvo.ViewObject) {
    // Assumes a previous auth middleware has set the userID.
    userID, _ := c.Get("userID") 
    // The ViewObject will need a Set method to support this.
    vo.Set("userID", userID)
}

func main() {
    // 2. Configure the enricher ONCE at application startup.
    // This tells the adaptor to use this logic for all subsequent `Bind` calls.
    adaptor.SetGlobalEnricher(AddUserInfo)

    router := gin.Default()
    // ... setup routes
}
```

### Step 2: Bind the ViewObject in Your Route

Use `adaptor.Bind()` as a middleware in your route. It handles the entire validation and enrichment process.

```go
func main() {
    // ... (enricher setup from above)

    router := gin.Default()
    router.Use(Authenticate()) // An auth middleware that sets the "userID"

    // 3. Use `adaptor.Bind` in your route.
    // It automatically validates the request and then calls the global enricher.
    router.POST("/signup", adaptor.Bind(signupVOProvider), func(c *gin.Context) {
        // 4. Access the final, validated, and enriched data.
        validatedData, _ := dvo.Get(c)

        // `validatedData` is a map[string]any that now contains:
        // - username (string)
        // - password (string)
        // - email (string)
        // - userID (from the enricher)

        c.JSON(http.StatusOK, gin.H{
            "message": "signup successful",
            "data":    validatedData,
        })
    })

    router.Run(":8080")
}
```

With this pattern, your routes are incredibly clean, and your core validation and enrichment logic is defined once in a central, reusable location.
