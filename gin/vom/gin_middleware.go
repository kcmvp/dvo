package vom

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/internal"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/tidwall/gjson"
)

// EnrichFunc defines a function type for enriching the validated data.
type EnrichFunc func(*gin.Context) map[string]any

var _enrich EnrichFunc
var once sync.Once

// SetGlobalEnricher sets a function to be called for enriching the validated data
// when validation is successful. This function is set only once.
func SetGlobalEnricher(enrich EnrichFunc) {
	once.Do(func() {
		_enrich = enrich
	})
}

// Bind creates a Gin middleware that validates the request body against a dvo.ViewObject.
// If validation is successful, the validated data is stored in the request context.
// If validation fails, it aborts the request with a 400 Bad Request status and an error message.
// It also allows for enriching the validated data using a previously set EnrichFunc function.
func Bind(vo *dvo.ViewObject) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get a fresh ValueObject instance for this request.
		bts := mo.TupleToResult[[]byte](io.ReadAll(c.Request.Body))
		if bts.IsError() {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": bts.Error().Error()})
			return
		}
		body := string(bts.MustGet())
		if !gjson.Valid(body) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}
		result := vo.Validate(body)
		if result.IsError() {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": result.Error().Error()})
			return
		}
		data := result.MustGet()
		if _enrich != nil {
			for k, v := range _enrich(c) {
				op := data.Get(k)
				lo.Assertf(op.IsPresent(), "property %s exiests", k)
				data.Set(k, v)
			}
		}
		// Store the validated object in the request's context for the main handler to use.
		ctx := context.WithValue(c.Request.Context(), internal.ViewObjectKey, data)
		c.Request = c.Request.WithContext(ctx)
		// Proceed to the next handler.
		c.Next()
	}
}

// ValueObject retrieves the validated ViewObject from the gin context.
// It returns nil if the object is not found.
func ValueObject(c *gin.Context) dvo.ValueObject {
	if val := c.Request.Context().Value(internal.ViewObjectKey); val != nil {
		if vo, ok := val.(dvo.ValueObject); ok {
			return vo
		}
	}
	return nil
}
