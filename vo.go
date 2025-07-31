package dvo

import (
	"context"
	"fmt"
	"github.com/samber/mo"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// A private type to prevent key collisions in context.
type validatedViewKeyType struct{}

// ValidatedViewKey is the key used to store the validated data map in the request context.
var ValidatedViewKey = validatedViewKeyType{}

var (
	// Statically assert that *Property[T] implements ViewProperty.
	_        ViewProperty = (*Property[string])(nil)
	registry              = make(map[string]*ViewObject)
	mu       sync.RWMutex
)

// JSONType is a constraint for the actual Go types we want to validate.
type JSONType interface {
	int | int64 | int32 | string | bool | time.Time
}

// Validator is a pure, type-safe function that receives an already-typed value.
type Validator[T JSONType] func(val T) error

// Document is a type-safe wrapper for the map of validated data.
type Document struct {
	values map[string]any
	ctx    context.Context
}

// String provides a type-safe way to retrieve a string value from the Document.
func (d *Document) String(key string) mo.Result[string] {
	v, ok := d.values[key]
	if !ok {
		return mo.Err[string](fmt.Errorf("key '%s' not found in validated document", key))
	}
	typedVal, ok := v.(string)
	if !ok {
		return mo.Err[string](fmt.Errorf("type mismatch for key '%s': expected string but got %T", key, v))
	}
	return mo.Ok(typedVal)
}

// Int provides a type-safe way to retrieve an int value from the Document.
func (d *Document) Int(key string) mo.Result[int] {
	v, ok := d.values[key]
	if !ok {
		return mo.Err[int](fmt.Errorf("key '%s' not found in validated document", key))
	}
	// JSON numbers are often float64, so we handle various numeric types.
	switch n := v.(type) {
	case int:
		return mo.Ok(n)
	case float64:
		return mo.Ok(int(n))
	case int32:
		return mo.Ok(int(n))
	case int64:
		return mo.Ok(int(n))
	}
	return mo.Err[int](fmt.Errorf("type mismatch for key '%s': expected a number but got %T", key, v))
}

// Bool provides a type-safe way to retrieve a bool value from the Document.
func (d *Document) Bool(key string) mo.Result[bool] {
	v, ok := d.values[key]
	if !ok {
		return mo.Err[bool](fmt.Errorf("key '%s' not found in validated document", key))
	}
	typedVal, ok := v.(bool)
	if !ok {
		return mo.Err[bool](fmt.Errorf("type mismatch for key '%s': expected bool but got %T", key, v))
	}
	return mo.Ok(typedVal)
}

// Date provides a type-safe way to retrieve a time.Time value from the Document.
func (d *Document) Date(key string) mo.Result[time.Time] {
	v, ok := d.values[key]
	if !ok {
		return mo.Err[time.Time](fmt.Errorf("key '%s' not found in validated document", key))
	}
	typedVal, ok := v.(time.Time)
	if !ok {
		return mo.Err[time.Time](fmt.Errorf("type mismatch for key '%s': expected time.Time but got %T", key, v))
	}
	return mo.Ok(typedVal)
}

// DocValidator is a function that validates the document using a type-safe Document.
type DocValidator func(doc *Document) error

type Enrich func(r *http.Request, doc *Document) error

// Policy defines a function that determines if an action should be taken for a request.
type Policy func(r *http.Request) bool

// ValidationErrors is a custom error type that holds a slice of validation errors.
type ValidationErrors []error

// Error implements the error interface, formatting all contained errors.
func (v *ValidationErrors) Error() string {
	if v == nil || len(*v) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("validation failed with the following errors:\n")
	for _, err := range *v {
		b.WriteString(fmt.Sprintf("- %s\n", err.Error()))
	}
	return b.String()
}

// Add appends a new error to the list if it's not nil.
func (v *ValidationErrors) Add(err error) {
	if err != nil {
		*v = append(*v, err)
	}
}

// Err returns the ValidationErrors as a single error if it contains any errors.
func (v *ValidationErrors) Err() error {
	if v == nil || len(*v) == 0 {
		return nil
	}
	return v
}

// Property now includes a 'required' flag to control validation behavior.
type Property[T JSONType] struct {
	name       string
	required   bool // This flag makes presence validation possible.
	validators []Validator[T]
}

// PropertyOption configures a Property using the functional options pattern.
type PropertyOption[T JSONType] func(*Property[T])

// Required is an option that marks a property as mandatory.
func Required[T JSONType]() PropertyOption[T] {
	return func(p *Property[T]) {
		p.required = true
	}
}

// WithValidators is an option to add validators to a property.
func WithValidators[T JSONType](validators ...Validator[T]) PropertyOption[T] {
	return func(p *Property[T]) {
		p.validators = append(p.validators, validators...)
	}
}

func (prop *Property[T]) isViewProperty() {}

func (prop *Property[T]) Name() string {
	return prop.name
}

// Validate now checks the 'required' flag and returns the typed value on success.
func (prop *Property[T]) Validate(json string) (any, error) {
	res := gjson.Get(json, prop.name)

	if !res.Exists() {
		if prop.required {
			return nil, fmt.Errorf("property '%s' is required", prop.name)
		}
		// Not required and not present, so it's valid.
		return nil, nil
	}

	// This is the DEFAULT TYPE VALIDATOR.
	typedValue := typed[T](res)
	if typedValue.IsError() {
		// If casting fails, it's a fundamental type mismatch error.
		return nil, fmt.Errorf("property '%s': %w", prop.name, typedValue.Error())
	}

	val := typedValue.MustGet()
	// If the type is correct, now run all additional registered validators on the value.
	errs := &ValidationErrors{}
	for _, validator := range prop.validators {
		if err := validator(val); err != nil {
			// Prefix the validator's error with the property name for context.
			errs.Add(fmt.Errorf("property '%s' %w", prop.name, err))
		}
	}

	if err := errs.Err(); err != nil {
		return nil, err
	}
	return val, nil
}

// typed is an unexported helper utility to convert a gjson.Result to a specific JSONType.
func typed[T JSONType](res gjson.Result) mo.Result[T] {
	var zero T
	switch any(zero).(type) {
	case string:
		if res.Type == gjson.String {
			return mo.Ok(any(res.String()).(T))
		}
	case bool:
		if res.Type == gjson.True || res.Type == gjson.False {
			return mo.Ok(any(res.Bool()).(T))
		}
	case int:
		if res.Type == gjson.Number {
			return mo.Ok(any(int(res.Int())).(T))
		}
	case int32:
		if res.Type == gjson.Number {
			return mo.Ok(any(int32(res.Int())).(T))
		}
	case int64:
		if res.Type == gjson.Number {
			return mo.Ok(any(res.Int()).(T))
		}
	case time.Time:
		if res.Type == gjson.String {
			t, err := time.Parse(time.RFC3339, res.String())
			if err == nil {
				return mo.Ok(any(t).(T))
			}
		}
	}
	return mo.Err[T](fmt.Errorf("type mismatch: expected %T but got JSON type %s", zero, res.Type))
}

// JSONProperty is a factory that accepts functional options for configuration.
func JSONProperty[T JSONType](name string, valType T, opts ...PropertyOption[T]) *Property[T] {
	p := &Property[T]{
		name: name,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type ViewProperty interface {
	Name() string
	Validate(json string) (any, error)
	isViewProperty()
}

// ViewObject now has a flag to control strict validation.
type ViewObject struct {
	name               string
	props              []ViewProperty
	allowUnknownFields bool // Default is false, meaning unknown fields are disallowed.
}

// ViewObjectOption configures a ViewObject using the functional options pattern.
type ViewObjectOption func(*ViewObject)

// WithProperties adds a list of properties to a ViewObject definition.
func WithProperties(props ...ViewProperty) ViewObjectOption {
	return func(vo *ViewObject) {
		vo.props = append(vo.props, props...)
	}
}

// AllowUnknownFields is an option to make the ViewObject accept JSON
// that contains properties not defined in the schema. Default behavior is to disallow.
func AllowUnknownFields() ViewObjectOption {
	return func(vo *ViewObject) {
		vo.allowUnknownFields = true
	}
}

// Register now uses the functional options pattern and validates the VO definition.
func Register(name string, opts ...ViewObjectOption) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		return fmt.Errorf("ViewObject with name '%s' already registered", name)
	}

	vo := &ViewObject{name: name}
	for _, opt := range opts {
		opt(vo)
	}

	// --- Enforcement Rules ---

	// 1. Enforce that a VO must have at least one property.
	if len(vo.props) == 0 {
		return fmt.Errorf("ViewObject '%s' must be registered with at least one property via WithProperties()", name)
	}

	// 2. Enforce that all property names are unique.
	propNames := make(map[string]struct{}, len(vo.props))
	for _, p := range vo.props {
		if _, exists := propNames[p.Name()]; exists {
			return fmt.Errorf("ViewObject '%s' has a duplicate property: %s", name, p.Name())
		}
		propNames[p.Name()] = struct{}{}
	}

	registry[name] = vo
	return nil
}

func VO(name string) mo.Result[*ViewObject] {
	mu.RLock()
	defer mu.RUnlock()
	vo, ok := registry[name]
	if ok {
		return mo.Ok(vo)
	}
	return mo.Err[*ViewObject](fmt.Errorf("ViewObject with name '%s' not found", name))
}

// Validate now accepts DocValidators at the call site and returns a Document on success.
func (vo *ViewObject) Validate(json string, docValidators ...DocValidator) (*Document, error) {
	// Step 1: Check for unknown fields if strict validation is enabled (default).
	if !vo.allowUnknownFields {
		unknownFieldErrs := &ValidationErrors{}
		knownProps := make(map[string]struct{}, len(vo.props))
		for _, p := range vo.props {
			knownProps[p.Name()] = struct{}{}
		}
		// This check only works for top-level JSON objects.
		root := gjson.Parse(json)
		if root.IsObject() {
			root.ForEach(func(key, value gjson.Result) bool {
				if _, ok := knownProps[key.String()]; !ok {
					unknownFieldErrs.Add(fmt.Errorf("unknown property '%s' found", key.String()))
				}
				return true // continue iterating
			})
		}

		// If unknown fields were found, fail fast.
		if err := unknownFieldErrs.Err(); err != nil {
			return nil, err
		}
	}

	// Step 2: Collect all property-level errors and build the validation context.
	propErrs := &ValidationErrors{}
	validatedMap := make(map[string]any)
	for _, prop := range vo.props {
		value, err := prop.Validate(json)
		if err != nil {
			propErrs.Add(err)
			continue
		}
		// Only add non-nil values to the map. This handles optional fields that are not present.
		if value != nil {
			validatedMap[prop.Name()] = value
		}
	}

	// If property-level errors exist, return them immediately.
	if err := propErrs.Err(); err != nil {
		return nil, err
	}

	// Step 3: Only if properties are valid, proceed to document-level validation with the context.
	doc := &Document{values: validatedMap}
	docErrs := &ValidationErrors{}
	for _, validator := range docValidators {
		docErrs.Add(validator(doc))
	}
	if err := docErrs.Err(); err != nil {
		return nil, err
	}

	return doc, nil
}

// validateProperties performs the first stage of validation on the request body data.
func (vo *ViewObject) validateProperties(ctx context.Context, body string) (*Document, error) {
	// --- Part 1: Property Validation ---
	if !vo.allowUnknownFields {
		unknownFieldErrs := &ValidationErrors{}
		knownProps := make(map[string]struct{}, len(vo.props))
		for _, p := range vo.props {
			knownProps[p.Name()] = struct{}{}
		}
		//@todo @2025-07-31 11:24:18 check property
		//for key := range bodyData {
		//	if _, ok := knownProps[key]; !ok {
		//		unknownFieldErrs.Add(fmt.Errorf("unknown property '%s' found in request body", key))
		//	}
		//}
		if err := unknownFieldErrs.Err(); err != nil {
			return nil, err
		}
	}

	propErrs := &ValidationErrors{}
	validatedProperties := make(map[string]any)
	for _, prop := range vo.props {
		value, err := prop.Validate(body)
		if err != nil {
			propErrs.Add(err)
			continue
		}
		if value != nil {
			validatedProperties[prop.Name()] = value
		}
	}
	if err := propErrs.Err(); err != nil {
		return nil, err
	}

	// Return the partially validated document, ready for enrichment.
	return &Document{values: validatedProperties, ctx: ctx}, nil
}

// --- Middleware ---

// Enricher pairs an enrichment function with a policy that controls its execution.
// It is unexported to hide the implementation detail from the public API.
type Enricher struct {
	policy Policy
	enric  Enrich
}

// Always returns a conditional enric that always runs.
func Always(enrich Enrich) Enricher {
	return Enricher{enric: enrich} // A nil policy means always run.
}

// When returns a conditional enric that only runs if the policy function returns true.
func When(policy Policy, enrich Enrich) Enricher {
	return Enricher{policy: policy, enric: enrich}
}

type middlewareConfig struct {
	enrichers     []Enricher
	docValidators []DocValidator
}

// MiddlewareOption configures the validation middleware.
type MiddlewareOption func(*middlewareConfig)

// WithEnrichers adds conditional enrichers to the validation pipeline,
// which are created using the Always() and When() helpers.
func WithEnrichers(enrichers ...Enricher) MiddlewareOption {
	return func(c *middlewareConfig) {
		c.enrichers = append(c.enrichers, enrichers...)
	}
}

// WithDocValidators adds document-level validators to the middleware.
func WithDocValidators(validators ...DocValidator) MiddlewareOption {
	return func(c *middlewareConfig) {
		c.docValidators = append(c.docValidators, validators...)
	}
}

// VOMiddleware returns a standard http.Handler middleware that validates a request.
func VOMiddleware(voName string, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	cfg := &middlewareConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vo, err := VO(voName).Get()
			if err != nil {
				http.Error(w, fmt.Sprintf("Internal Server Error: %s", err.Error()), http.StatusInternalServerError)
				return
			}

			// --- Pipeline Step 0: Extract Body ---
			//bodyData := make(map[string]any)
			var body []byte
			if r.Body != nil && r.Body != http.NoBody {
				body, err = io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Bad Request: Could not read body", http.StatusBadRequest)
					return
				}
				//if len(body) > 0 {
				//	if err := json.Unmarshal(body, &bodyData); err != nil {
				//		http.Error(w, "Bad Request: Invalid JSON", http.StatusBadRequest)
				//		return
				//	}
				//}
			}

			// --- Pipeline Step 1: Property Validation ---
			doc, err := vo.validateProperties(r.Context(), string(body))
			if err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"error": "validation failed", "details": "%s"}`, err.Error())
				return
			}

			// --- Pipeline Step 2: Conditional Enricher ---
			enricherErrs := &ValidationErrors{}
			for _, ce := range cfg.enrichers {
				// Run if there is no policy (Always) or if the policy passes.
				if ce.policy == nil || ce.policy(r) {
					if err := ce.enric(r, doc); err != nil {
						enricherErrs.Add(err)
					}
				}
			}
			if err := enricherErrs.Err(); err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"error": "validation failed", "details": "%s"}`, err.Error())
				return
			}

			// --- Pipeline Step 3: Document Validation ---
			docErrs := &ValidationErrors{}
			for _, validator := range cfg.docValidators {
				docErrs.Add(validator(doc))
			}
			if err := docErrs.Err(); err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"error": "validation failed", "details": "%s"}`, err.Error())
				return
			}

			// --- Success: Store final document and proceed ---
			ctx := context.WithValue(r.Context(), ValidatedViewKey, doc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
