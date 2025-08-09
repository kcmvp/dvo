package dvo

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kcmvp/dvo/constraint"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestTyped(t *testing.T) {
	t.Run("integers", func(t *testing.T) {
		// Test cases for integer types
		tests := []struct {
			name                string
			json                string
			want                mo.Result[any]
			targetType          any
			expectedErr         error
			expectedErrContains string
		}{
			{
				name:       "int_ok",
				json:       `{"value": 123}`,
				want:       mo.Ok(any(int(123))),
				targetType: int(0),
			},
			{
				name:        "int_from_string_fail",
				json:        `{"value": "123"}`,
				targetType:  int(0),
				expectedErr: constraint.ErrTypeMismatch,
			},
			{
				name:        "int_overflow",
				json:        fmt.Sprintf(`{"value": %d1}`, math.MaxInt), // Overflow
				targetType:  int(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:       "int8_ok",
				json:       `{"value": 127}`,
				want:       mo.Ok(any(int8(127))),
				targetType: int8(0),
			},
			{
				name:        "int8_overflow",
				json:        `{"value": 128}`,
				targetType:  int8(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:       "int16_ok",
				json:       `{"value": 32767}`,
				want:       mo.Ok(any(int16(32767))),
				targetType: int16(0),
			},
			{
				name:        "int16_overflow",
				json:        `{"value": 32768}`,
				targetType:  int16(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:       "int32_ok",
				json:       `{"value": 2147483647}`,
				want:       mo.Ok(any(int32(2147483647))),
				targetType: int32(0),
			},
			{
				name:        "int32_overflow",
				json:        `{"value": 2147483648}`,
				targetType:  int32(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:       "int64_ok",
				json:       fmt.Sprintf(`{"value": %d}`, int64(math.MaxInt64)),
				want:       mo.Ok(any(int64(math.MaxInt64))),
				targetType: int64(0),
			},
			{
				name:        "int64_overflow",
				json:        `{"value": 9223372036854775808}`,
				targetType:  int64(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:        "int64_underflow",
				json:        `{"value": -9223372036854775809}`,
				targetType:  int64(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:        "int_from_float_fail",
				json:        `{"value": 123.45}`,
				targetType:  int(0),
				expectedErr: constraint.ErrTypeMismatch,
			},
			{
				name:        "int_from_bool_fail",
				json:        `{"value": true}`,
				targetType:  int(0),
				expectedErr: constraint.ErrTypeMismatch,
			},
			{
				name:                "int_invalid_number_format",
				json:                `{"value": 1.2.3}`, // Invalid number format
				targetType:          int(0),
				expectedErrContains: "could not parse number",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				res := gjson.Get(tc.json, "value")
				var got mo.Result[any]
				switch tc.targetType.(type) {
				case int:
					got = mo.TupleToResult[any](typed[int](res).Get())
				case int8:
					got = mo.TupleToResult[any](typed[int8](res).Get())
				case int16:
					got = mo.TupleToResult[any](typed[int16](res).Get())
				case int32:
					got = mo.TupleToResult[any](typed[int32](res).Get())
				case int64:
					got = mo.TupleToResult[any](typed[int64](res).Get())
				default:
					t.Fatalf("unhandled test type: %T", tc.targetType)
				}
				if tc.expectedErr != nil {
					require.True(t, got.IsError(), "expected an error but got none")
					require.ErrorIs(t, got.Error(), tc.expectedErr, "did not get expected error type")
				} else if tc.expectedErrContains != "" {
					require.True(t, got.IsError(), "expected an error but got none")
					require.Contains(t, got.Error().Error(), tc.expectedErrContains, "error message does not contain expected text")
				} else {
					require.False(t, got.IsError(), "got unexpected error: %v", got.Error())
					require.Equal(t, tc.want.MustGet(), got.MustGet())
				}
			})
		}
	})
	t.Run("unsigned integers", func(t *testing.T) {
		tests := []struct {
			name                string
			json                string
			want                mo.Result[any]
			targetType          any
			expectedErr         error
			expectedErrContains string
		}{
			{
				name:       "uint_ok",
				json:       `{"value": 123}`,
				want:       mo.Ok(any(uint(123))),
				targetType: uint(0),
			},
			{
				name:        "uint_from_string_fail",
				json:        `{"value": "123"}`,
				targetType:  uint(0),
				expectedErr: constraint.ErrTypeMismatch,
			},
			{
				name:        "uint_negative_fail",
				json:        `{"value": -1}`,
				targetType:  uint(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:       "uint8_ok",
				json:       `{"value": 255}`,
				want:       mo.Ok(any(uint8(255))),
				targetType: uint8(0),
			},
			{
				name:        "uint8_overflow",
				json:        `{"value": 256}`,
				targetType:  uint8(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:       "uint16_ok",
				json:       `{"value": 65535}`,
				want:       mo.Ok(any(uint16(65535))),
				targetType: uint16(0),
			},
			{
				name:        "uint16_overflow",
				json:        `{"value": 65536}`,
				targetType:  uint16(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:       "uint32_ok",
				json:       `{"value": 4294967295}`,
				want:       mo.Ok(any(uint32(4294967295))),
				targetType: uint32(0),
			},
			{
				name:        "uint32_overflow",
				json:        `{"value": 4294967296}`,
				targetType:  uint32(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:       "uint64_ok",
				json:       fmt.Sprintf(`{"value": %d}`, uint64(math.MaxUint64)),
				want:       mo.Ok(any(uint64(math.MaxUint64))),
				targetType: uint64(0),
			},
			{
				name:        "uint64_overflow",
				json:        fmt.Sprintf(`{"value":%s}`, "18446744073709551616"), // MaxUint64 + 1
				targetType:  uint64(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:        "uint64_overflow_from_large_number",
				json:        `{"value": 18446744073709551616}`,
				targetType:  uint64(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:        "uint_from_float_fail",
				json:        `{"value": 123.45}`,
				targetType:  uint(0),
				expectedErr: constraint.ErrTypeMismatch,
			},
			{
				name:        "uint_from_bool_fail",
				json:        `{"value": true}`,
				targetType:  uint(0),
				expectedErr: constraint.ErrTypeMismatch,
			},
			{
				name:                "uint_invalid_number_format",
				json:                `{"value": 1.2.3}`, // Invalid number format
				targetType:          uint(0),
				expectedErrContains: "could not parse number",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				res := gjson.Get(tc.json, "value")
				var got mo.Result[any]
				switch tc.targetType.(type) {
				case uint:
					got = mo.TupleToResult[any](typed[uint](res).Get())
				case uint8:
					got = mo.TupleToResult[any](typed[uint8](res).Get())
				case uint16:
					got = mo.TupleToResult[any](typed[uint16](res).Get())
				case uint32:
					got = mo.TupleToResult[any](typed[uint32](res).Get())
				case uint64:
					got = mo.TupleToResult[any](typed[uint64](res).Get())
				default:
					t.Fatalf("unhandled test type: %T", tc.targetType)
				}

				if tc.expectedErr != nil {
					require.True(t, got.IsError(), "expected an error but got none")
					require.ErrorIs(t, got.Error(), tc.expectedErr, "did not get expected error type")
				} else if tc.expectedErrContains != "" {
					require.True(t, got.IsError(), "expected an error but got none")
					require.Contains(t, got.Error().Error(), tc.expectedErrContains, "error message does not contain expected text")
				} else {
					require.False(t, got.IsError(), "got unexpected error: %v", got.Error())
					require.Equal(t, tc.want.MustGet(), got.MustGet())
				}
			})
		}
	})
	t.Run("floats", func(t *testing.T) {
		tests := []struct {
			name        string
			json        string
			want        mo.Result[any]
			targetType  any
			expectedErr bool
		}{
			{
				name:       "float32_ok",
				json:       `{"value": 123.45}`,
				want:       mo.Ok(any(float32(123.45))),
				targetType: float32(0),
			},
			{
				name:        "float32_overflow",
				json:        `{"value": 3.5e+38}`,
				targetType:  float32(0),
				expectedErr: true,
			},
			{
				name:       "float64_ok",
				json:       `{"value": 1.7976931348623157e+308}`,
				want:       mo.Ok(any(float64(1.7976931348623157e+308))),
				targetType: float64(0),
			},
			{
				name:        "float64_overflow",
				json:        `{"value": 1.8e+308}`,
				want:        mo.Ok(any(math.Inf(1))),
				targetType:  float64(0),
				expectedErr: false,
			},
			{
				name:        "float_from_string_fail",
				json:        `{"value": "123.45"}`,
				targetType:  float64(0),
				expectedErr: true,
			},
			{
				name:        "float_from_bool_fail",
				json:        `{"value": true}`,
				targetType:  float64(0),
				expectedErr: true,
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				res := gjson.Get(tc.json, "value")
				var got mo.Result[any]
				switch tc.targetType.(type) {
				case float32:
					got = mo.TupleToResult[any](typed[float32](res).Get())
				case float64:
					got = mo.TupleToResult[any](typed[float64](res).Get())
				default:
					t.Fatalf("unhandled test type: %T", tc.targetType)
				}

				if tc.expectedErr {
					require.True(t, got.IsError(), "expected an error but got none")
				} else {
					require.False(t, got.IsError(), "got unexpected error: %v", got.Error())
					gotVal := got.MustGet()
					wantVal := tc.want.MustGet()
					isInf := false
					if f, ok := wantVal.(float64); ok {
						if math.IsInf(f, 0) {
							isInf = true
						}
					} else if f, ok := wantVal.(float32); ok {
						if math.IsInf(float64(f), 0) {
							isInf = true
						}
					}
					if isInf {
						require.Equal(t, wantVal, gotVal)
					} else {
						require.InDelta(t, wantVal, gotVal, 1e-9)
					}
				}
			})
		}
	})
	t.Run("booleans", func(t *testing.T) {
		tests := []struct {
			name        string
			json        string
			want        mo.Result[any]
			targetType  any
			expectedErr error
		}{
			{
				name:       "bool_ok",
				json:       `{"value": true}`,
				want:       mo.Ok(any(true)),
				targetType: bool(false),
			},
			{
				name:        "bool_from_string_fail",
				json:        `{"value": "true"}`,
				targetType:  bool(false),
				expectedErr: constraint.ErrTypeMismatch,
			},
			{
				name:        "bool_from_number_fail",
				json:        `{"value": 1}`,
				targetType:  bool(false),
				expectedErr: constraint.ErrTypeMismatch,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				res := gjson.Get(tc.json, "value")
				var got mo.Result[any]
				switch tc.targetType.(type) {
				case bool:
					got = mo.TupleToResult[any](typed[bool](res).Get())
				default:
					t.Fatalf("unhandled test type: %T", tc.targetType)
				}

				if tc.expectedErr != nil {
					require.True(t, got.IsError(), "expected an error but got none")
					require.ErrorIs(t, got.Error(), tc.expectedErr, "did not get expected error type")
				} else {
					require.False(t, got.IsError(), "got unexpected error: %v", got.Error())
					require.Equal(t, tc.want.MustGet(), got.MustGet())
				}
			})
		}
	})
	t.Run("time", func(t *testing.T) {
		// Test cases for time.Time
		now := time.Now()
		tests := []struct {
			name        string
			json        string
			want        mo.Result[time.Time]
			expectedErr bool
		}{
			{
				name: "time_ok_rfc3339",
				json: fmt.Sprintf(`{"value": "%s"}`, now.Format(time.RFC3339)),
				want: mo.Ok(now.Truncate(time.Second)),
			},
			{
				name: "time_ok_date_only",
				json: `{"value": "2023-01-15"}`,
				want: mo.Ok(time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)),
			},
			{
				name:        "time_invalid_format",
				json:        `{"value": "15-01-2023"}`,
				expectedErr: true,
			},
			{
				name:        "time_from_number_fail",
				json:        `{"value": 1234567890}`,
				expectedErr: true,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				res := gjson.Get(tc.json, "value")
				got := typed[time.Time](res)

				if tc.expectedErr {
					require.True(t, got.IsError(), "expected an error but got none")
				} else {
					require.False(t, got.IsError(), "got unexpected error: %v", got.Error())
					// Truncate to remove monotonic clock readings for comparison
					require.WithinDuration(t, tc.want.MustGet(), got.MustGet(), time.Second, "time values are not equal")
				}
			})
		}
	})
}

func TestViewField_Validate(t *testing.T) {
	tests := []struct {
		name        string
		field       *ViewField[string]
		json        string
		wantResult  mo.Result[string]
		wantFound   bool
		expectedErr error
	}{
		{
			name:       "required_field_present_and_valid",
			field:      Field[string]("name")(),
			json:       `{"name": "kcmvp"}`,
			wantResult: mo.Ok("kcmvp"),
			wantFound:  true,
		},
		{
			name:        "required_field_missing",
			field:       Field[string]("name")(),
			json:        `{}`,
			wantFound:   false,
			expectedErr: constraint.ErrRequired,
		},
		{
			name:       "optional_field_missing",
			field:      Field[string]("name")().Optional(),
			json:       `{}`,
			wantResult: mo.Ok(""), // Zero value
			wantFound:  false,
		},
		{
			name:       "optional_field_present",
			field:      Field[string]("name")().Optional(),
			json:       `{"name": "kcmvp"}`,
			wantResult: mo.Ok("kcmvp"),
			wantFound:  true,
		},
		{
			name:        "type_mismatch",
			field:       Field[string]("name")(),
			json:        `{"name": 123}`,
			wantFound:   true,
			expectedErr: constraint.ErrTypeMismatch,
		},
		{
			name:       "custom_validator_success",
			field:      Field[string]("password")(constraint.MinLength(5)),
			json:       `{"password": "valid_password"}`,
			wantResult: mo.Ok("valid_password"),
			wantFound:  true,
		},
		{
			name:        "custom_validator_failure",
			field:       Field[string]("password")(constraint.MinLength(6)),
			json:        `{"password": "short"}`,
			wantFound:   true,
			expectedErr: constraint.ErrLengthMin,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotResult, gotFound := tc.field.Validate(tc.json)
			require.Equal(t, tc.wantFound, gotFound)
			if tc.expectedErr != nil {
				require.True(t, gotResult.IsError(), "expected an error but got none")
				require.ErrorIs(t, gotResult.Error(), tc.expectedErr, "did not get expected error type")
			} else {
				require.False(t, gotResult.IsError(), "got unexpected error: %v", gotResult.Error())
				require.Equal(t, tc.wantResult.MustGet(), gotResult.MustGet())
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name  string
		err   *validationError
		check func(t *testing.T, got string)
	}{
		{
			name: "nil error",
			err:  nil,
			check: func(t *testing.T, got string) {
				require.Equal(t, "", got)
			},
		},
		{
			name: "empty error",
			err:  &validationError{},
			check: func(t *testing.T, got string) {
				require.Equal(t, "", got)
			},
		},
		{
			name: "one error",
			err: &validationError{
				errors: map[string]error{
					"field1": errors.New("error 1"),
				},
			},
			check: func(t *testing.T, got string) {
				require.Equal(t, "validation failed with the following errors:- error 1", got)
			},
		},
		{
			name: "multiple errors",
			err: &validationError{
				errors: map[string]error{
					"field1": errors.New("error 1"),
					"field2": errors.New("error 2"),
				},
			},
			check: func(t *testing.T, got string) {
				require.True(t, strings.HasPrefix(got, "validation failed with the following errors:"))
				require.Contains(t, got, "- error 1")
				require.Contains(t, got, "- error 2")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			tt.check(t, got)
		})
	}
}

func TestValidationError_add(t *testing.T) {
	t.Run("add to nil error map", func(t *testing.T) {
		e := &validationError{}
		require.Nil(t, e.errors)
		err := errors.New("some error")
		e.add("field1", err)
		require.NotNil(t, e.errors)
		require.Equal(t, err, e.errors["field1"])
	})

	t.Run("add to existing error map", func(t *testing.T) {
		e := &validationError{
			errors: make(map[string]error),
		}
		err1 := errors.New("error 1")
		e.add("field1", err1)
		require.Equal(t, err1, e.errors["field1"])

		err2 := errors.New("error 2")
		e.add("field2", err2)
		require.Equal(t, err2, e.errors["field2"])
		require.Len(t, e.errors, 2)
	})

	t.Run("overwrite existing error", func(t *testing.T) {
		e := &validationError{
			errors: make(map[string]error),
		}
		err1 := errors.New("error 1")
		e.add("field1", err1)
		require.Equal(t, err1, e.errors["field1"])

		errOverwrite := errors.New("overwrite error")
		e.add("field1", errOverwrite)
		require.Equal(t, errOverwrite, e.errors["field1"])
		require.Len(t, e.errors, 1)
	})

	t.Run("add nil error", func(t *testing.T) {
		e := &validationError{}
		e.add("field1", nil)
		require.Nil(t, e.errors)
		require.Len(t, e.errors, 0)

		e.errors = make(map[string]error)
		e.add("field2", nil)
		require.Len(t, e.errors, 0)
	})
}

func TestValidationError_err(t *testing.T) {
	tests := []struct {
		name    string
		err     *validationError
		wantErr bool
	}{
		{
			name:    "nil validationError",
			err:     nil,
			wantErr: false,
		},
		{
			name:    "validationError with nil errors map",
			err:     &validationError{errors: nil},
			wantErr: false,
		},
		{
			name:    "validationError with empty errors map",
			err:     &validationError{errors: make(map[string]error)},
			wantErr: false,
		},
		{
			name: "validationError with one error",
			err: &validationError{
				errors: map[string]error{
					"field1": errors.New("error 1"),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.err.err()
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.err, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestViewField_validate(t *testing.T) {
	tests := []struct {
		name      string
		field     viewField
		json      string
		wantValue any
		wantFound bool
		wantErr   error
	}{
		{
			name:      "required_field_present_and_valid",
			field:     Field[string]("name")(),
			json:      `{"name": "gopher"}`,
			wantValue: "gopher",
			wantFound: true,
			wantErr:   nil,
		},
		{
			name:      "required_field_present_but_invalid",
			field:     Field[string]("name")(constraint.MinLength(10)),
			json:      `{"name": "gopher"}`,
			wantValue: nil,
			wantFound: true,
			wantErr:   constraint.ErrLengthMin,
		},
		{
			name:      "required_field_missing",
			field:     Field[string]("name")(),
			json:      `{}`,
			wantValue: nil,
			wantFound: false,
			wantErr:   constraint.ErrRequired,
		},
		{
			name:      "optional_field_missing",
			field:     Field[string]("name")().Optional(),
			json:      `{}`,
			wantValue: nil,
			wantFound: false,
			wantErr:   nil,
		},
		{
			name:      "optional_field_present_and_valid",
			field:     Field[string]("name")().Optional(),
			json:      `{"name": "gopher"}`,
			wantValue: "gopher",
			wantFound: true,
			wantErr:   nil,
		},
		{
			name:      "optional_field_present_but_invalid",
			field:     Field[string]("name")(constraint.MinLength(10)).Optional(),
			json:      `{"name": "gopher"}`,
			wantValue: nil,
			wantFound: true,
			wantErr:   constraint.ErrLengthMin,
		},
		{
			name:      "type_mismatch",
			field:     Field[int]("age")(),
			json:      `{"age": "not-an-age"}`,
			wantValue: nil,
			wantFound: true,
			wantErr:   constraint.ErrTypeMismatch,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotValue, gotFound, gotErr := tc.field.validate(tc.json)

			require.Equal(t, tc.wantFound, gotFound)
			require.Equal(t, tc.wantValue, gotValue)

			if tc.wantErr != nil {
				require.Error(t, gotErr)
				require.ErrorIs(t, gotErr, tc.wantErr)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestWithFields(t *testing.T) {
	tests := []struct {
		name        string
		fields      []viewField
		shouldPanic bool
	}{
		{
			name:        "no fields",
			fields:      []viewField{},
			shouldPanic: false,
		},
		{
			name:        "one field",
			fields:      []viewField{Field[string]("name")()},
			shouldPanic: false,
		},
		{
			name: "multiple unique fields",
			fields: []viewField{
				Field[string]("name")(),
				Field[int]("age")(),
			},
			shouldPanic: false,
		},
		{
			name: "duplicate field name",
			fields: []viewField{
				Field[string]("name")(),
				Field[int]("name")(),
			},
			shouldPanic: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.shouldPanic {
				require.Panics(t, func() {
					WithFields(tc.fields...)
				})
			} else {
				vo := WithFields(tc.fields...)
				require.NotNil(t, vo)
				require.Len(t, vo.fields, len(tc.fields))
			}
		})
	}
}

func TestViewObject_AllowUnknownFields(t *testing.T) {
	tests := []struct {
		name          string
		vo            *ViewObject
		json          string
		allowUnknown  bool
		expectErr     bool
		errContains   string
		expectVal     string
		expectPresent bool
	}{
		{
			name:         "Default behavior: unknown fields not allowed, should error",
			vo:           WithFields(Field[string]("name")()),
			json:         `{"name": "gopher", "extra": "field"}`,
			allowUnknown: false,
			expectErr:    true,
			errContains:  "unknown field 'extra'",
		},
		{
			name:          "AllowUnknownFields enabled: unknown fields allowed, should not error",
			vo:            WithFields(Field[string]("name")()),
			json:          `{"name": "gopher", "extra": "field"}`,
			allowUnknown:  true,
			expectErr:     false,
			expectVal:     "gopher",
			expectPresent: true,
		},
		{
			name:         "No unknown fields: should not error (default)",
			vo:           WithFields(Field[string]("name")()),
			json:         `{"name": "gopher"}`,
			allowUnknown: false,
			expectErr:    false,
		},
		{
			name:         "No unknown fields: should not error (allowed)",
			vo:           WithFields(Field[string]("name")()),
			json:         `{"name": "gopher"}`,
			allowUnknown: true,
			expectErr:    false,
		},
		{
			name:         "Corner case: empty JSON, should not error",
			vo:           WithFields(Field[string]("name")()),
			json:         `{}`,
			allowUnknown: false,
			expectErr:    true, // required field is missing
			errContains:  "name is required",
		},
		{
			name:         "Corner case: empty ViewObject, should not error",
			vo:           WithFields(),
			json:         `{"name": "gopher"}`,
			allowUnknown: true,
			expectErr:    false,
		},
		{
			name:         "Corner case: empty ViewObject, unknown fields disallowed, should error",
			vo:           WithFields(),
			json:         `{"name": "gopher"}`,
			allowUnknown: false,
			expectErr:    true,
			errContains:  "unknown field 'name'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.allowUnknown {
				// Test that the method is chainable
				returnedVo := tc.vo.AllowUnknownFields()
				require.Same(t, tc.vo, returnedVo, "AllowUnknownFields should be chainable")
			}

			res := tc.vo.Validate(tc.json)

			if tc.expectErr {
				require.True(t, res.IsError(), "expected an error but got none")
				require.Contains(t, res.Error().Error(), tc.errContains, "error message does not contain expected text")
			} else {
				require.False(t, res.IsError(), "got unexpected error: %v", res.Error())
				if tc.expectPresent {
					val, ok := res.MustGet().String("name").Get()
					require.True(t, ok)
					require.Equal(t, tc.expectVal, val)
				}
			}
		})
	}
}

func TestEndToEnd(t *testing.T) {
	// Define the ViewObject with various field types and constraints
	userView := WithFields(
		Field[string]("name")(constraint.MinLength(3)),
		Field[int]("age")(constraint.Between(18, 120)),
		Field[string]("email")(constraint.Email()).Optional(),
		Field[bool]("isActive")(),
		Field[time.Time]("createdAt")().Optional(),
		Field[float64]("rating")(constraint.Between(0.0, 5.0)),
		Field[string]("department")().Optional(),
		Field[string]("username")(constraint.CharSetOnly(constraint.LowerCaseChar)),
		Field[string]("nickname")(), // No validator for 'not contains substring' yet
		Field[string]("countryCode")(constraint.ExactLength(2)),
		Field[string]("homepage")(constraint.URL()).Optional(),
		Field[string]("status")(constraint.OneOf[string]("active", "inactive", "pending")),
		Field[string]("tags")(constraint.Match(`tag_*`)),
		Field[float64]("salary")(constraint.Gt(0.0)),
		// New fields for all JSON types
		Field[int8]("level")(constraint.Between[int8](1, 100)),
		Field[int16]("score")(constraint.Gt[int16](0)),
		Field[int32]("views")(constraint.Gte[int32](0)),
		Field[int64]("balance")(constraint.Gte[int64](0)),
		Field[uint]("flags")(),
		Field[uint8]("version")(),
		Field[uint16]("build")(),
		Field[uint32]("instanceId")(),
		Field[uint64]("nonce")(),
		Field[float32]("ratio")(constraint.Between[float32](0.0, 1.0)),
	)

	// Test cases
	tests := []struct {
		name    string
		isValid bool
		check   func(t *testing.T, vo ValueObject)
	}{
		{
			name:    "valid user",
			isValid: true,
			check: func(t *testing.T, vo ValueObject) {
				// String
				name, ok := vo.String("name").Get()
				require.True(t, ok)
				require.Equal(t, "John Doe", name)
				email, ok := vo.String("email").Get()
				require.True(t, ok)
				require.Equal(t, "john.doe@example.com", email)
				username, ok := vo.String("username").Get()
				require.True(t, ok)
				require.Equal(t, "johndoe", username)
				nickname, ok := vo.String("nickname").Get()
				require.True(t, ok)
				require.Equal(t, "Johnny", nickname)
				countryCode, ok := vo.String("countryCode").Get()
				require.True(t, ok)
				require.Equal(t, "US", countryCode)
				tags, ok := vo.String("tags").Get()
				require.True(t, ok)
				require.Equal(t, "tag_go,developer,testing", tags)
				status, ok := vo.String("status").Get()
				require.True(t, ok)
				require.Equal(t, "active", status)

				// Optional String not present
				_, ok = vo.String("department").Get()
				require.False(t, ok)

				// Bool
				isActive, ok := vo.Bool("isActive").Get()
				require.True(t, ok)
				require.True(t, isActive)

				// Time (optional, not present)
				_, ok = vo.Time("createdAt").Get()
				require.False(t, ok)

				// Numbers
				age, ok := vo.Int("age").Get()
				require.True(t, ok)
				require.Equal(t, 30, age)
				rating, ok := vo.Float64("rating").Get()
				require.True(t, ok)
				require.Equal(t, 4.5, rating)
				salary, ok := vo.Float64("salary").Get()
				require.True(t, ok)
				require.Equal(t, 50000.0, salary)
				level, ok := vo.Int8("level").Get()
				require.True(t, ok)
				require.Equal(t, int8(10), level)
				score, ok := vo.Int16("score").Get()
				require.True(t, ok)
				require.Equal(t, int16(1000), score)
				views, ok := vo.Int32("views").Get()
				require.True(t, ok)
				require.Equal(t, int32(100000), views)
				balance, ok := vo.Int64("balance").Get()
				require.True(t, ok)
				require.Equal(t, int64(1000000000), balance)
				flags, ok := vo.Uint("flags").Get()
				require.True(t, ok)
				require.Equal(t, uint(4294967295), flags)
				version, ok := vo.Uint8("version").Get()
				require.True(t, ok)
				require.Equal(t, uint8(255), version)
				build, ok := vo.Uint16("build").Get()
				require.True(t, ok)
				require.Equal(t, uint16(65535), build)
				instanceId, ok := vo.Uint32("instanceId").Get()
				require.True(t, ok)
				require.Equal(t, uint32(4294967295), instanceId)
				nonce, ok := vo.Uint64("nonce").Get()
				require.True(t, ok)
				require.Equal(t, uint64(18446744073709551615), nonce)
				ratio, ok := vo.Float32("ratio").Get()
				require.True(t, ok)
				require.Equal(t, float32(0.5), ratio)
			},
		},
		{
			name:    "invalid rating",
			isValid: false,
		},
		{
			name:    "missing required field",
			isValid: false,
		},
		{
			name:    "valid user with all optional fields",
			isValid: true,
			check: func(t *testing.T, vo ValueObject) {
				// Check optional fields that are present
				createdAt, ok := vo.Time("createdAt").Get()
				require.True(t, ok)
				expectedTime, _ := time.Parse(time.RFC3339, "2024-01-01T12:00:00Z")
				require.WithinDuration(t, expectedTime, createdAt, time.Second)

				department, ok := vo.String("department").Get()
				require.True(t, ok)
				require.Equal(t, "Security", department)

				homepage, ok := vo.String("homepage").Get()
				require.True(t, ok)
				require.Equal(t, "https://matrix.com", homepage)

				// Also check all other fields to ensure they are correctly parsed
				name, ok := vo.String("name").Get()
				require.True(t, ok)
				require.Equal(t, "John Doe", name)
				age, ok := vo.Int("age").Get()
				require.True(t, ok)
				require.Equal(t, 30, age)
				isActive, ok := vo.Bool("isActive").Get()
				require.True(t, ok)
				require.True(t, isActive)
				level, ok := vo.Int8("level").Get()
				require.True(t, ok)
				require.Equal(t, int8(10), level)
				score, ok := vo.Int16("score").Get()
				require.True(t, ok)
				require.Equal(t, int16(1000), score)
				views, ok := vo.Int32("views").Get()
				require.True(t, ok)
				require.Equal(t, int32(100000), views)
				balance, ok := vo.Int64("balance").Get()
				require.True(t, ok)
				require.Equal(t, int64(1000000000), balance)
				flags, ok := vo.Uint("flags").Get()
				require.True(t, ok)
				require.Equal(t, uint(4294967295), flags)
				version, ok := vo.Uint8("version").Get()
				require.True(t, ok)
				require.Equal(t, uint8(255), version)
				build, ok := vo.Uint16("build").Get()
				require.True(t, ok)
				require.Equal(t, uint16(65535), build)
				instanceId, ok := vo.Uint32("instanceId").Get()
				require.True(t, ok)
				require.Equal(t, uint32(4294967295), instanceId)
				nonce, ok := vo.Uint64("nonce").Get()
				require.True(t, ok)
				require.Equal(t, uint64(18446744073709551615), nonce)
				ratio, ok := vo.Float32("ratio").Get()
				require.True(t, ok)
				require.Equal(t, float32(0.5), ratio)
			},
		},
		{
			name:    "valid user without optional email",
			isValid: true,
			check: func(t *testing.T, vo ValueObject) {
				_, ok := vo.String("email").Get()
				require.False(t, ok, "email should not be present")

				// Check other fields to ensure they are still valid
				name, ok := vo.String("name").Get()
				require.True(t, ok)
				require.Equal(t, "John Doe", name)
				age, ok := vo.Int("age").Get()
				require.True(t, ok)
				require.Equal(t, 30, age)
				isActive, ok := vo.Bool("isActive").Get()
				require.True(t, ok)
				require.True(t, isActive)
			},
		},
		{
			name:    "invalid username charset",
			isValid: false,
		},
		{
			name:    "invalid countryCode length",
			isValid: false,
		},
		{
			name:    "invalid tags pattern",
			isValid: false,
		},
		{
			name:    "invalid homepage url",
			isValid: false,
		},
		{
			name:    "invalid status",
			isValid: false,
		},
		{
			name:    "invalid salary",
			isValid: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Read json from testdata
			jsonPath := filepath.Join("testdata", fmt.Sprintf("%s.json", strings.ReplaceAll(tc.name, " ", "_")))
			jsonData, err := os.ReadFile(jsonPath)
			require.NoError(t, err, "failed to read test data file")

			// Validate
			res := userView.Validate(string(jsonData))

			if tc.isValid {
				require.False(t, res.IsError(), "expected validation to succeed, but it failed with: %v", res.Error())
				vo := res.MustGet()
				require.NotNil(t, vo)
				if tc.check != nil {
					tc.check(t, vo)
				}
			} else {
				require.True(t, res.IsError(), "expected validation to fail, but it succeeded")
			}
		})
	}
}

func TestValueObject_GetSet(t *testing.T) {
	vo := make(valueObject)

	// 1. Test Set and Get for a new value
	vo.Set("name", "gopher")
	nameOpt := vo.Get("name")
	require.True(t, nameOpt.IsPresent(), "expected 'name' to be present")
	name, ok := nameOpt.Get()
	require.True(t, ok)
	require.Equal(t, "gopher", name)

	// 2. Test Get for a non-existent value
	ageOpt := vo.Get("age")
	require.False(t, ageOpt.IsPresent(), "expected 'age' to be absent")

	// 3. Test Set to overwrite an existing value
	vo.Set("name", "gopher-overwritten")
	newNameOpt := vo.Get("name")
	require.True(t, newNameOpt.IsPresent(), "expected 'name' to be present after overwrite")
	newName, ok := newNameOpt.Get()
	require.True(t, ok)
	require.Equal(t, "gopher-overwritten", newName)
}

func TestValueObject_Set_LogsOnOverwrite(t *testing.T) {
	// 1. Setup a buffer to capture log output
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, nil)
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	// 2. Create a valueObject and trigger the overwrite
	vo := make(valueObject)
	vo.Set("name", "gopher")
	vo.Set("name", "gopher-overwritten") // This should log

	// 3. Assert the log output
	logOutput := logBuf.String()
	require.Contains(t, logOutput, "overwrite existing property", "log should contain overwrite message")
	require.Contains(t, logOutput, "property=name", "log should contain the property name")
}

func TestValueObject_MustMethods(t *testing.T) {
	now := time.Now()
	vo := valueObject{
		"my_string":  "hello",
		"my_int":     int(-1),
		"my_int8":    int8(-8),
		"my_int16":   int16(-16),
		"my_int32":   int32(-32),
		"my_int64":   int64(-64),
		"my_uint":    uint(1),
		"my_uint8":   uint8(8),
		"my_uint16":  uint16(16),
		"my_uint32":  uint32(32),
		"my_uint64":  uint64(64),
		"my_float32": float32(32.32),
		"my_float64": float64(64.64),
		"my_bool":    true,
		"my_time":    now,
	}

	t.Run("successful gets", func(t *testing.T) {
		require.Equal(t, "hello", vo.MstString("my_string"))
		require.Equal(t, int(-1), vo.MstInt("my_int"))
		require.Equal(t, int8(-8), vo.MstInt8("my_int8"))
		require.Equal(t, int16(-16), vo.MstInt16("my_int16"))
		require.Equal(t, int32(-32), vo.MstInt32("my_int32"))
		require.Equal(t, int64(-64), vo.MstInt64("my_int64"))
		require.Equal(t, uint(1), vo.MstUint("my_uint"))
		require.Equal(t, uint8(8), vo.MstUint8("my_uint8"))
		require.Equal(t, uint16(16), vo.MstUint16("my_uint16"))
		require.Equal(t, uint32(32), vo.MstUint32("my_uint32"))
		require.Equal(t, uint64(64), vo.MstUint64("my_uint64"))
		require.Equal(t, float32(32.32), vo.MstFloat32("my_float32"))
		require.Equal(t, float64(64.64), vo.MstFloat64("my_float64"))
		require.Equal(t, true, vo.MstBool("my_bool"))
		require.Equal(t, now, vo.MstTime("my_time"))
	})

	t.Run("panic on missing key", func(t *testing.T) {
		require.Panics(t, func() { vo.MstString("nonexistent") })
		require.Panics(t, func() { vo.MstInt("nonexistent") })
		require.Panics(t, func() { vo.MstInt8("nonexistent") })
		require.Panics(t, func() { vo.MstInt16("nonexistent") })
		require.Panics(t, func() { vo.MstInt32("nonexistent") })
		require.Panics(t, func() { vo.MstInt64("nonexistent") })
		require.Panics(t, func() { vo.MstUint("nonexistent") })
		require.Panics(t, func() { vo.MstUint8("nonexistent") })
		require.Panics(t, func() { vo.MstUint16("nonexistent") })
		require.Panics(t, func() { vo.MstUint32("nonexistent") })
		require.Panics(t, func() { vo.MstUint64("nonexistent") })
		require.Panics(t, func() { vo.MstFloat32("nonexistent") })
		require.Panics(t, func() { vo.MstFloat64("nonexistent") })
		require.Panics(t, func() { vo.MstBool("nonexistent") })
		require.Panics(t, func() { vo.MstTime("nonexistent") })
	})
}

func TestField_PanicOnDuplicateValidator(t *testing.T) {
	t.Run("duplicate validator", func(t *testing.T) {
		require.PanicsWithValue(t, "dvo: duplicate validator 'min_length' for field 'password'", func() {
			Field[string]("password")(constraint.MinLength(5), constraint.MinLength(10))
		})
	})
}
