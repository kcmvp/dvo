
package internal

import (
	"fmt"
)

// PathParamFunc is a function type that extracts all path parameters from a generic context as a map.
type PathParamFunc func(ctx any) map[string]string

// QueryParamFunc is a function type that extracts all query parameters from a generic context as a map.
type QueryParamFunc func(ctx any) map[string][]string

// Unify is a higher-order function that returns a new function to extract and merge URL parameters.
// It enforces a strict policy: it will panic if a naming conflict is found between a path and query parameter.
// It also intelligently handles multi-value query parameters, preserving them as slices.
func Unify(pathFunc PathParamFunc, queryFunc QueryParamFunc) func(ctx any) (map[string]any, error) {
	return func(ctx any) (map[string]any, error) {
		data := make(map[string]any)
		// 1. Extract path parameters.
		pathParams := pathFunc(ctx)
		for k, v := range pathParams {
			data[k] = v
		}

		// 2. Extract query parameters and check for conflicts before merging.
		queryParams := queryFunc(ctx)
		for key := range queryParams {
			if _, exists := pathParams[key]; exists {
				panic(fmt.Sprintf("dvo: naming conflict detected in API design. The key '%s' is used as both a path parameter and a query parameter. Please rename one to avoid ambiguity.", key))
			}
		}

		// 3. Merge query parameters with intelligent type handling.
		for key, values := range queryParams {
			if len(values) == 1 {
				data[key] = values[0]
			} else if len(values) > 1 {
				data[key] = values
			}
			// if len(values) == 0, the key is ignored.
		}

		return data, nil
	}
}
