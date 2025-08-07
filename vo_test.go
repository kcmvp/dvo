package dvo

import (
	"fmt"
	"github.com/kcmvp/dvo/constraint"
	"math"
	"testing"
	"time"

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
