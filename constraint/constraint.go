package constraint

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/samber/mo"
	"github.com/tidwall/match"

	"github.com/samber/lo"
)

type charSet int

type Number interface {
	uint | uint8 | uint16 | uint32 | uint64 | int | int8 | int16 | int32 | int64 | float32 | float64
}

// JSONType is a constraint for the actual Go types we want to validate.
type JSONType interface {
	Number | string | time.Time | bool
}

type Validator[T JSONType] func(v T) error
type ValidateFunc[T JSONType] func() (string, Validator[T])

const (
	LowerCaseChar charSet = iota
	UpperCaseChar
	NumberChar
	SpecialChar
)

var (
	LowerCaseCharSet = string(lo.LowerCaseLettersCharset)
	UpperCaseCharSet = string(lo.UpperCaseLettersCharset)
	NumberCharSet    = string(lo.NumbersCharset)
	SpecialCharSet   = string(lo.SpecialCharset)
)

var (
	ErrIntegerOverflow = errors.New("integer overflow")
	ErrTypeMismatch    = errors.New("type mismatch")
	ErrRequired        = errors.New("is required but not found")

	ErrLengthMin     = errors.New("length must be at least")
	ErrLengthMax     = errors.New("length must be at most")
	ErrLengthExact   = errors.New("length must be exactly")
	ErrLengthBetween = errors.New("length must be between")

	ErrMin = errors.New("must be at least")
	ErrMax = errors.New("must be at most")

	ErrCharSetOnly   = errors.New("can only contain characters from")
	ErrCharSetAny    = errors.New("must contain at least one character from")
	ErrCharSetAll    = errors.New("not contains chars from")
	ErrCharSetNo     = errors.New("must not contain any characters from")
	ErrNotMatch      = errors.New("not match pattern")
	ErrNotValidEmail = errors.New("not valid email address")
	ErrNotValidURL   = errors.New("not valid url")
	ErrNotOneOf      = errors.New("value must be one of")
	ErrMustGt        = errors.New("must be greater than")
	ErrMustGte       = errors.New("must be greater than or equal to")
	ErrMustLt        = errors.New("must be less than")
	ErrMustLte       = errors.New("must be less than or equal to")
	ErrMustBetween   = errors.New("must be between")
	ErrMustBeTrue    = errors.New("must be true")
	ErrMustBeFalse   = errors.New("must be false")
)

// value is a private helper to get the character set and its descriptive name.
func (set charSet) value() (chars string, name string) {
	switch set {
	case LowerCaseChar:
		return LowerCaseCharSet, "lower case letters"
	case UpperCaseChar:
		return UpperCaseCharSet, "upper case letters"
	case NumberChar:
		return NumberCharSet, "numbers"
	case SpecialChar:
		return SpecialCharSet, "special characters"
	default:
		panic("unhandled default case in charSet.value()")
	}
}

// --- String Validators ---

// MinLength validates that a string's length is at least the specified minimum.
func MinLength(min int) ValidateFunc[string] {
	return func() (string, Validator[string]) {
		return "min_length", func(str string) error {
			return lo.Ternary(len(str) < min, fmt.Errorf("%w %d ", ErrLengthMin, min), nil)
		}
	}
}

// MaxLength validates that a string's length is at most the specified maximum.
func MaxLength(max int) ValidateFunc[string] {
	return func() (string, Validator[string]) {
		return "max_length", func(str string) error {
			return lo.Ternary(len(str) > max, fmt.Errorf("%w %d ", ErrLengthMax, max), nil)
		}
	}
}

// ExactLength validates that a string's length is exactly the specified length.
func ExactLength(length int) ValidateFunc[string] {
	return func() (string, Validator[string]) {
		return "exact_length", func(str string) error {
			return lo.Ternary(len(str) != length, fmt.Errorf("%w %d characters", ErrLengthExact, length), nil)
		}
	}

}

// LengthBetween validates that a string's length is within a given range (inclusive).
func LengthBetween(min, max int) ValidateFunc[string] {
	return func() (string, Validator[string]) {
		return "length_between", func(str string) error {
			length := len(str)
			return lo.Ternary(length < min || length > max, fmt.Errorf("%w %d and %d characters", ErrLengthBetween, min, max), nil)
		}
	}
}

// CharSetOnly validates that a string only contains characters from the specified character sets.
func CharSetOnly(charSets ...charSet) ValidateFunc[string] {
	return func() (string, Validator[string]) {
		return "only_contains", func(str string) error {
			var allChars strings.Builder
			var names []string
			for _, set := range charSets {
				chars, name := set.value()
				allChars.WriteString(chars)
				names = append(names, name)
			}
			for _, r := range str {
				if !strings.ContainsRune(allChars.String(), r) {
					return fmt.Errorf("%w: %s", ErrCharSetOnly, strings.Join(names, ", "))
				}
			}
			return nil
		}
	}
}

// CharSetAny validates that a string contains at least one character from any of the specified character sets.
func CharSetAny(charSets ...charSet) ValidateFunc[string] {
	return func() (string, Validator[string]) {
		return "contains_any", func(str string) error {
			var allChars strings.Builder
			var names []string
			for _, set := range charSets {
				chars, name := set.value()
				allChars.WriteString(chars)
				names = append(names, name)
			}
			return lo.Ternary(!strings.ContainsAny(allChars.String(), str), fmt.Errorf("%w: %s", ErrCharSetAny, strings.Join(names, ", ")), nil)
		}
	}
}

// CharSetAll validates that a string contains at least one character from each of the specified character sets.
func CharSetAll(charSets ...charSet) ValidateFunc[string] {
	return func() (string, Validator[string]) {
		return "contains_all", func(str string) error {
			for _, set := range charSets {
				chars, name := set.value()
				if !strings.ContainsAny(chars, str) {
					return fmt.Errorf("%w: %s", ErrCharSetAll, name)
				}
			}
			return nil
		}
	}

}

// CharSetNo validates that a string does not contain any characters from the specified character sets.
func CharSetNo(charSets ...charSet) ValidateFunc[string] {
	return func() (string, Validator[string]) {
		return "not_contains", func(str string) error {
			for _, set := range charSets {
				chars, name := set.value()
				if strings.ContainsAny(str, chars) {
					return fmt.Errorf("%s: %s", ErrCharSetNo, name)
				}
			}
			return nil
		}
	}
}

// Match validates that a string matches a given pattern.
// The pattern can include wildcards:
//   - `*`: matches any sequence of non-separator characters.
//   - `?`: matches any single non-separator character.
//
// Example: Match("foo*") will match "foobar", "foo", etc.
func Match(pattern string) ValidateFunc[string] {
	lo.Assertf(match.IsPattern(pattern), "invalid pattern `%s`: `?` stands for one character, `*` stands for any number of characters", pattern)
	return func() (string, Validator[string]) {
		return "match", func(str string) error {
			return lo.Ternary(!match.Match(str, pattern), fmt.Errorf("%w %s", ErrNotMatch, pattern), nil)
		}
	}
}

// Email validates that a string is a valid email address.
func Email() ValidateFunc[string] {
	return func() (string, Validator[string]) {
		return "email", func(str string) error {
			return lo.Ternary(mo.TupleToResult[*mail.Address](mail.ParseAddress(str)).IsError(), fmt.Errorf("%w:%s", ErrNotValidEmail, str), nil)
		}
	}
}

// URL validates that a string is a valid URL.
func URL() ValidateFunc[string] {
	return func() (string, Validator[string]) {
		return "url", func(str string) error {
			rs := mo.TupleToResult[*url.URL](url.Parse(str))
			errRs := rs.IsError() || rs.MustGet().Scheme == "" || rs.MustGet().Host == ""
			return lo.Ternary(errRs, fmt.Errorf("%w: %s", ErrNotValidURL, str), nil)
		}
	}
}

// --- Generic and Comparison types.Validators ---

// OneOf validates that a value is one of the allowed values.
// This works for any comparable type in JSONType (string, bool, all numbers).
func OneOf[T JSONType](allowed ...T) ValidateFunc[T] {
	return func() (string, Validator[T]) {
		return "one_of", func(val T) error {
			return lo.Ternary(!lo.Contains(allowed, val), fmt.Errorf("%w:%v", ErrNotOneOf, allowed), nil)
		}
	}
}

// Gt validates that a value is greater than the specified minimum.
func Gt[T Number | time.Time](min T) ValidateFunc[T] {
	return func() (string, Validator[T]) {
		return "gt", func(val T) error {
			return lo.Ternary(!isGreaterThan(val, min), fmt.Errorf("%w %v", ErrMustGt, min), nil)
		}
	}
}

// Gte validates that a value is greater than or equal to the specified minimum.
func Gte[T Number | time.Time](min T) ValidateFunc[T] {
	return func() (string, Validator[T]) {
		return "gte", func(val T) error {
			return lo.Ternary(isLessThan(val, min), fmt.Errorf("%w %v", ErrMustGte, min), nil)
		}
	}
}

// Lt validates that a value is less than the specified maximum.
func Lt[T Number | time.Time](max T) ValidateFunc[T] {
	return func() (string, Validator[T]) {
		return "lt", func(val T) error {
			return lo.Ternary(!isLessThan(val, max), fmt.Errorf("%w %v", ErrMustLt, max), nil)
		}
	}
}

// Lte validates that a value is less than or equal to the specified maximum.
func Lte[T Number | time.Time](max T) ValidateFunc[T] {
	return func() (string, Validator[T]) {
		return "lte", func(val T) error {
			return lo.Ternary(isGreaterThan(val, max), fmt.Errorf("%w %v", ErrMustLte, max), nil)
		}
	}
}

// Between validates that a value is within a given range (inclusive of min and max).
func Between[T Number | time.Time](min, max T) ValidateFunc[T] {
	return func() (string, Validator[T]) {
		return "between", func(val T) error {
			return lo.Ternary(isLessThan(val, min) || isGreaterThan(val, max), fmt.Errorf("%w %v and %v", ErrMustBetween, min, max), nil)
		}
	}
}

// Min validates that a number or time.Time is greater than or equal to a minimum value.
func Min[T Number | time.Time](min T) ValidateFunc[T] {
	return func() (string, Validator[T]) {
		return "min_number_date", func(val T) error {
			return lo.Ternary(isLessThan(val, min), fmt.Errorf("%w %v", ErrMin, min), nil)
		}
	}
}

// Max validates that a number or time.Time is less than or equal to a maximum value.
func Max[T Number | time.Time](max T) ValidateFunc[T] {
	return func() (string, Validator[T]) {
		return "max_number_date", func(val T) error {
			return lo.Ternary(isGreaterThan(val, max), fmt.Errorf("%w %v", ErrMax, max), nil)
		}
	}
}

// --- Boolean Validators ---

// BeTrue validates that a boolean value is true.
func BeTrue() ValidateFunc[bool] {
	return func() (string, Validator[bool]) {
		return "be_true", func(b bool) error {
			return lo.Ternary(!b, ErrMustBeTrue, nil)
		}
	}
}

// BeFalse validates that a boolean value is false.
func BeFalse() ValidateFunc[bool] {
	return func() (string, Validator[bool]) {
		return "be_false", func(b bool) error {
			return lo.Ternary(b, ErrMustBeFalse, nil)
		}
	}
}

// isGreaterThan is a helper function that compares two values of type Number or time.Time
// and returns true if 'a' is strictly greater than 'b'.
// It handles different numeric types and time.Time by type assertion.
func isGreaterThan[T Number | time.Time](a, b T) bool {
	switch v := any(a).(type) {
	case time.Time:
		return v.After(any(b).(time.Time))
	case int:
		return v > any(b).(int)
	case int8:
		return v > any(b).(int8)
	case int16:
		return v > any(b).(int16)
	case int32:
		return v > any(b).(int32)
	case int64:
		return v > any(b).(int64)
	case uint:
		return v > any(b).(uint)
	case uint8:
		return v > any(b).(uint8)
	case uint16:
		return v > any(b).(uint16)
	case uint32:
		return v > any(b).(uint32)
	case uint64:
		return v > any(b).(uint64)
	case float32:
		return v > any(b).(float32)
	case float64:
		return v > any(b).(float64)
	}
	return false
}

// isLessThan is a helper function that compares two values of type Number or time.Time
// and returns true if 'a' is strictly less than 'b'.
// It handles different numeric types and time.Time by type assertion.

func isLessThan[T Number | time.Time](a, b T) bool {
	switch v := any(a).(type) {
	case time.Time:
		return v.Before(any(b).(time.Time))
	case int:
		return v < any(b).(int)
	case int8:
		return v < any(b).(int8)
	case int16:
		return v < any(b).(int16)
	case int32:
		return v < any(b).(int32)
	case int64:
		return v < any(b).(int64)
	case uint:
		return v < any(b).(uint)
	case uint8:
		return v < any(b).(uint8)
	case uint16:
		return v < any(b).(uint16)
	case uint32:
		return v < any(b).(uint32)
	case uint64:
		return v < any(b).(uint64)
	case float32:
		return v < any(b).(float32)
	case float64:
		return v < any(b).(float64)
	}
	return false
}
