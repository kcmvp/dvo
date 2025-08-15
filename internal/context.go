package internal

// A private type to prevent key collisions in context.
type viewObjectKeyType struct{}

// ViewObjectKey is the key used to store the validated valueObject map in the request context.
var ViewObjectKey = viewObjectKeyType{}
