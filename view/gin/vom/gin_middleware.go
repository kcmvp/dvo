package vom

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/kcmvp/dvo/internal"
	"github.com/kcmvp/dvo/view"
	"github.com/samber/lo"
	"github.com/samber/mo"
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

func urlParams(ctx *gin.Context) map[string]string {
	params := lo.Associate(ctx.Params, func(item gin.Param) (string, string) {
		return item.Key, item.Value
	})
	for name, values := range ctx.Request.URL.Query() {
		lo.Assertf(len(values) == 1, "query parameter '%s' has multiple values, which is not supported", name)
		_, ok := params[name]
		lo.Assertf(!ok, "path parameter and query parameter have conflicting names: '%s'", name)
		params[name] = values[0]
	}
	return params
}

// Bind creates a Gin middleware that validates the request body against a dvo.Schema.
// If validation is successful, the validated data is stored in the request context.
// If validation fails, it aborts the request with a 400 Bad Request status and an error message.
// It also allows for enriching the validated data using a previously set EnrichFunc function.
func Bind(schema *view.Schema) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Get a fresh ValueObject instance for this request.
		bts := mo.TupleToResult[[]byte](io.ReadAll(ctx.Request.Body))
		if bts.IsError() {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": bts.Error().Error()})
			return
		}
		body := string(bts.MustGet())
		result := schema.Validate(body, urlParams(ctx))
		if result.IsError() {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": result.Error().Error()})
			return
		}
		data := result.MustGet()

		if _enrich != nil {
			for k, v := range _enrich(ctx) {
				data.Add(k, v)
			}
		}
		// Store the validated object in the request's context for the main handler to use.
		nCtx := context.WithValue(ctx.Request.Context(), internal.ViewObjectKey, data)
		ctx.Request = ctx.Request.WithContext(nCtx)
		// Proceed to the next handler.
		ctx.Next()
	}
}

// ValueObject retrieves the validated ValueObject from the gin context.
// It returns nil if the object is not found.
func ValueObject(c *gin.Context) view.ValueObject {
	if val := c.Request.Context().Value(internal.ViewObjectKey); val != nil {
		if vo, ok := val.(view.ValueObject); ok {
			return vo
		}
	}
	return nil
}
