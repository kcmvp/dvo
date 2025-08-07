package constraint

import "time"

type Number interface {
	uint | uint8 | uint16 | uint32 | uint64 | int | int8 | int16 | int32 | int64 | float32 | float64
}

// JSONType is a constraint for the actual Go types we want to validate.
type JSONType interface {
	Number | string | time.Time | bool
}

type Validator[T JSONType] func(v T) error
type ValidateFunc[T JSONType] func() (string, Validator[T])
