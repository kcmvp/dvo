package validator

import (
	"fmt"
	"github.com/kcmvp/dvo"
	"github.com/tidwall/match"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/samber/lo"
)

type CharSet int

const (
	LowerCaseChar CharSet = iota
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

// value is a private helper to get the character set and its descriptive name.
func (set CharSet) value() (chars string, name string) {
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
		panic("unhandled default case in CharSet.value()")
	}
}

// --- String Validators ---

// MinLength validates that a string's length is at least the specified minimum.
func MinLength(min int) func(string) error {
	return func(str string) error {
		if len(str) < min {
			return fmt.Errorf("length must be at least %d characters", min)
		}
		return nil
	}

}

// MaxLength validates that a string's length is at most the specified maximum.
func MaxLength(max int) func(string) error {
	return func(str string) error {
		if len(str) > max {
			return fmt.Errorf("length must be at most %d characters", max)
		}
		return nil
	}

}

// ExactLength validates that a string's length is exactly the specified length.
func ExactLength(max int) func(string) error {
	return func(str string) error {
		if len(str) != max {
			return fmt.Errorf("length must be exactly %d characters", max)
		}
		return nil
	}
}

// LengthBetween validates that a string's length is within a given range (inclusive).
func LengthBetween(min, max int) func(string) error {
	return func(str string) error {
		length := len(str)
		if length < min || length > max {
			return fmt.Errorf("length must be between %d and %d characters", min, max)
		}
		return nil
	}
}

// OnlyContains validates that a string only contains characters from the specified character sets.
func OnlyContains(charSets ...CharSet) func(string) error {
	return func(str string) error {
		var allChars strings.Builder
		var names []string
		for _, set := range charSets {
			chars, name := set.value()
			allChars.WriteString(chars)
			names = append(names, name)
		}
		for _, r := range str {
			if !strings.ContainsRune(allChars.String(), r) {
				return fmt.Errorf("can only contain characters from: %s", strings.Join(names, ", "))
			}
		}
		return nil
	}

}

// ContainsAny validates that a string contains at least one character from any of the specified character sets.
func ContainsAny(charSets ...CharSet) func(string) error {
	return func(str string) error {
		var allChars strings.Builder
		var names []string
		for _, set := range charSets {
			chars, name := set.value()
			allChars.WriteString(chars)
			names = append(names, name)
		}
		if !strings.ContainsAny(str, allChars.String()) {
			return fmt.Errorf("must contain at least one character from: %s", strings.Join(names, ", "))
		}
		return nil
	}
}

// ContainsAll validates that a string contains at least one character from *each* of the specified character sets.
func ContainsAll(charSets ...CharSet) func(string) error {
	return func(str string) error {
		for _, set := range charSets {
			chars, name := set.value()
			if !strings.ContainsAny(str, chars) {
				return fmt.Errorf("must contain at least one character from: %s", name)
			}
		}
		return nil
	}
}

// NotContains validates that a string does not contain any characters from the specified character sets.
func NotContains(charSets ...CharSet) func(string) error {
	return func(str string) error {
		for _, set := range charSets {
			chars, name := set.value()
			if strings.ContainsAny(str, chars) {
				return fmt.Errorf("must not contain any characters from: %s", name)
			}
		}
		return nil
	}
}

// Match validates that a string matches a given pattern.
// The pattern can include wildcards:
//   - `*`: matches any sequence of non-separator characters.
//   - `?`: matches any single non-separator character.
//
// Example: Match("foo*") will match "foobar", "foo", etc.
func Match(pattern string) func(string) error {
	if !match.IsPattern(pattern) {
		panic(fmt.Sprintf("invalid pattern provided to Match validator: %s", pattern))
	}
	return func(str string) error {
		if !match.Match(str, pattern) {
			return fmt.Errorf("does not match required pattern")
		}
		return nil
	}
}

// Email validates that a string is a valid email address.
func Email() func(string) error {
	return func(str string) error {
		_, err := mail.ParseAddress(str)
		if err != nil {
			return fmt.Errorf("is not a valid email address")
		}
		return nil
	}
}

// URL validates that a string is a valid URL.
func URL() func(string) error {
	return func(str string) error {
		u, err := url.Parse(str)
		if err != nil {
			return fmt.Errorf("is not a valid URL")
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("is not a valid URL")
		}
		return nil
	}
}

// --- Generic and Comparison Validators ---

// OneOf validates that a value is one of the allowed values.
// This works for any comparable type in JSONType (string, bool, all numbers).
func OneOf[T comparable](allowed []T) func(T) error {
	return func(val T) error {
		if !lo.Contains(allowed, val) {
			return fmt.Errorf("value must be one of %v", allowed)
		}
		return nil
	}
}

// Gt validates that a value is greater than the specified minimum.
func Gt[T dvo.Number | time.Time](min T) func(T) error {
	return func(val T) error {
		if !isGreaterThan(val, min) {
			return fmt.Errorf("must be greater than %v", min)
		}
		return nil
	}
}

// Gte validates that a value is greater than or equal to the specified minimum.
func Gte[T dvo.Number | time.Time](min T) func(T) error {
	return func(val T) error {
		if isLessThan(val, min) {
			return fmt.Errorf("must be greater than or equal to %v", min)
		}
		return nil
	}
}

// Lt validates that a value is less than the specified maximum.
func Lt[T dvo.Number | time.Time](max T) func(T) error {
	return func(val T) error {
		if !isLessThan(val, max) {
			return fmt.Errorf("must be less than %v", max)
		}
		return nil
	}
}

// Lte validates that a value is less than or equal to the specified maximum.
func Lte[T dvo.Number | time.Time](max T) func(T) error {
	return func(val T) error {
		if isGreaterThan(val, max) {
			return fmt.Errorf("must be less than or equal to %v", max)
		}
		return nil
	}
}

// Between validates that a value is within a given range (inclusive of min and max).
func Between[T dvo.Number | time.Time](min, max T) func(T) error {
	return func(val T) error {
		if isLessThan(val, min) || isGreaterThan(val, max) {
			return fmt.Errorf("must be between %v and %v", min, max)
		}
		return nil
	}
}

// --- Boolean Validators ---

// BeTrue validates that a boolean value is true.
func BeTrue() func(bool) error {
	return func(b bool) error {
		if !b {
			return fmt.Errorf("must be true")
		}
		return nil
	}
}

// BeFalse validates that a boolean value is false.
func BeFalse() func(bool) error {
	return func(b bool) error {
		if b {
			return fmt.Errorf("must be false")
		}
		return nil
	}
}

// isGreaterThan is a helper function that compares two values of type Number or time.Time
// and returns true if 'a' is strictly greater than 'b'.
// It handles different numeric types and time.Time by type assertion.
func isGreaterThan[T dvo.Number | time.Time](a, b T) bool {
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

func isLessThan[T dvo.Number | time.Time](a, b T) bool {
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
	case float32:
		return v < any(b).(float32)
	case float64:
		return v < any(b).(float64)
	}
	return false
}
