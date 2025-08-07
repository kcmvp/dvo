package dvo

import (
	"errors"
	"fmt"
	"math"
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
			name        string
			json        string
			want        mo.Result[any]
			targetType  any
			expectedErr error
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
				json:        "{\"value\": 9223372036854775808}", // MaxInt64 + 1
				targetType:  int64(0),
				expectedErr: constraint.ErrIntegerOverflow,
			},
			{
				name:        "int_from_float_fail",
				json:        `{"value": 123.45}`,
				targetType:  int(0),
				expectedErr: constraint.ErrTypeMismatch,
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
				json:        fmt.Sprintf(`{"value": %e}`, float64(math.MaxFloat32)*2),
				targetType:  float32(0),
				expectedErr: true,
			},
			{
				name:       "float64_ok",
				json:       `{"value": 1.7976931348623157e+308}`,
				want:       mo.Ok(any(float64(1.7976931348623157e+308))),
				targetType: float64(0),
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
					require.InDelta(t, tc.want.MustGet(), got.MustGet(), 1e-9)
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

func TestValidationError_Add(t *testing.T) {
	t.Run("add to nil error map", func(t *testing.T) {
		e := &validationError{}
		require.Nil(t, e.errors)
		err := errors.New("some error")
		e.Add("field1", err)
		require.NotNil(t, e.errors)
		require.Equal(t, err, e.errors["field1"])
	})

	t.Run("add to existing error map", func(t *testing.T) {
		e := &validationError{
			errors: make(map[string]error),
		}
		err1 := errors.New("error 1")
		e.Add("field1", err1)
		require.Equal(t, err1, e.errors["field1"])

		err2 := errors.New("error 2")
		e.Add("field2", err2)
		require.Equal(t, err2, e.errors["field2"])
		require.Len(t, e.errors, 2)
	})

	t.Run("overwrite existing error", func(t *testing.T) {
		e := &validationError{
			errors: make(map[string]error),
		}
		err1 := errors.New("error 1")
		e.Add("field1", err1)
		require.Equal(t, err1, e.errors["field1"])

		errOverwrite := errors.New("overwrite error")
		e.Add("field1", errOverwrite)
		require.Equal(t, errOverwrite, e.errors["field1"])
		require.Len(t, e.errors, 1)
	})

	t.Run("add nil error", func(t *testing.T) {
		e := &validationError{}
		e.Add("field1", nil)
		require.Nil(t, e.errors)
		require.Len(t, e.errors, 0)

		e.errors = make(map[string]error)
		e.Add("field2", nil)
		require.Len(t, e.errors, 0)
	})
}

func TestValidationError_Err(t *testing.T) {
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
			err := tt.err.Err()
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
