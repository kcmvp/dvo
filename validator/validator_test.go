package validator

import (
	"testing"
	"time"
)

func TestMinLength(t *testing.T) {
	tests := []struct {
		name    string
		min     int
		str     string
		wantErr bool
	}{
		{"too short", 5, "abc", true},
		{"exact length", 5, "abcde", false},
		{"longer", 5, "abcdef", false},
		{"empty string below min", 5, "", true},
		{"empty string at min 0", 0, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := MinLength(tt.min)()
			if err := v(tt.str); (err != nil) != tt.wantErr {
				t.Errorf("MinLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMaxLength(t *testing.T) {
	tests := []struct {
		name    string
		max     int
		str     string
		wantErr bool
	}{
		{"too long", 5, "abcdef", true},
		{"exact length", 5, "abcde", false},
		{"shorter", 5, "abc", false},
		{"empty string", 5, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := MaxLength(tt.max)()
			if err := v(tt.str); (err != nil) != tt.wantErr {
				t.Errorf("MaxLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExactLength(t *testing.T) {
	tests := []struct {
		name    string
		len     int
		str     string
		wantErr bool
	}{
		{"too short", 5, "abc", true},
		{"too long", 5, "abcdef", true},
		{"exact length", 5, "abcde", false},
		{"empty string want 0", 0, "", false},
		{"empty string want 5", 5, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := ExactLength(tt.len)()
			if err := v(tt.str); (err != nil) != tt.wantErr {
				t.Errorf("ExactLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLengthBetween(t *testing.T) {
	tests := []struct {
		name    string
		min     int
		max     int
		str     string
		wantErr bool
	}{
		{"too short", 3, 5, "ab", true},
		{"too long", 3, 5, "abcdef", true},
		{"min length", 3, 5, "abc", false},
		{"max length", 3, 5, "abcde", false},
		{"in between", 3, 5, "abcd", false},
		{"empty string too short", 3, 5, "", true},
		{"empty string in range", 0, 5, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := LengthBetween(tt.min, tt.max)()
			if err := v(tt.str); (err != nil) != tt.wantErr {
				t.Errorf("LengthBetween() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOnlyContains(t *testing.T) {
	tests := []struct {
		name     string
		charSets []CharSet
		str      string
		wantErr  bool
	}{
		{"only lower", []CharSet{LowerCaseChar}, "abc", false},
		{"only lower with number", []CharSet{LowerCaseChar}, "abc1", true},
		{"only lower and number", []CharSet{LowerCaseChar, NumberChar}, "abc1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := OnlyContains(tt.charSets...)()
			if err := v(tt.str); (err != nil) != tt.wantErr {
				t.Errorf("OnlyContains() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		charSets []CharSet
		str      string
		wantErr  bool
	}{
		{"contains lower", []CharSet{LowerCaseChar}, "abc", false},
		{"contains lower and number", []CharSet{LowerCaseChar, NumberChar}, "abc1", false},
		{"contains only number", []CharSet{LowerCaseChar, NumberChar}, "123", false},
		{"contains none", []CharSet{LowerCaseChar}, "123", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := ContainsAny(tt.charSets...)()
			if err := v(tt.str); (err != nil) != tt.wantErr {
				t.Errorf("ContainsAny() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainsAll(t *testing.T) {
	tests := []struct {
		name     string
		charSets []CharSet
		str      string
		wantErr  bool
	}{
		{"contains lower and number", []CharSet{LowerCaseChar, NumberChar}, "abc1", false},
		{"contains only lower", []CharSet{LowerCaseChar, NumberChar}, "abc", true},
		{"contains only number", []CharSet{LowerCaseChar, NumberChar}, "123", true},
		{"contains none", []CharSet{LowerCaseChar, NumberChar}, "ABC", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := ContainsAll(tt.charSets...)()
			if err := v(tt.str); (err != nil) != tt.wantErr {
				t.Errorf("ContainsAll() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNotContains(t *testing.T) {
	tests := []struct {
		name     string
		charSets []CharSet
		str      string
		wantErr  bool
	}{
		{"contains lower", []CharSet{LowerCaseChar}, "abc", true},
		{"contains number", []CharSet{NumberChar}, "abc1", true},
		{"does not contain special", []CharSet{SpecialChar}, "abc1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := NotContains(tt.charSets...)()
			if err := v(tt.str); (err != nil) != tt.wantErr {
				t.Errorf("NotContains() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		str     string
		wantErr bool
	}{
		{"match", "a.*c", "abc", true},
		{"match", "a.*c", "adefbc", true},
		{"no match", "a.*c", "abd", true},
		{"match with ?", "a?c", "ac", true},
		{"no match with ?", "a?c", "abc", false},
		{"no match with *", "a*c", "addddadaadc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := Match(tt.pattern)()
			if err := v(tt.str); (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMatch_PanicOnInvalidPattern(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic but was expected to")
		}
	}()
	// This call should panic because the regex pattern is invalid.
	_ = Match("[")
}

func TestEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid with subdomain", "test@mail.example.com", false},
		{"valid with plus alias", "test+alias@example.com", false},
		{"invalid email", "test", true},
		{"invalid email", "test@", true},
		{"@example.com", "@example.com", true},
		{"no at sign", "testexample.com", true},
		{"multiple at signs", "test@exa@mple.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := Email()()
			if err := v(tt.email); (err != nil) != tt.wantErr {
				t.Errorf("Email() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid url", "http://example.com", false},
		{"valid url with path", "http://example.com/path", false},
		{"valid url with path https", "https://example.com/path", false},
		{"valid url with port", "http://example.com:8080", false},
		{"valid url with query", "https://example.com?a=1&b=2", false},
		{"valid ftp url", "ftp://example.com", false},
		{"invalid url01", "example.com", true},
		{"invalid url02", "http://", true},
		{"malformed url", "://a.b", true},
		{"url with no host", "http:///path", true},
		{"url with spaces", "http://exa mple.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := URL()()
			if err := v(tt.url); (err != nil) != tt.wantErr {
				t.Errorf("URL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOneOf(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		tests := []struct {
			name    string
			allowed []string
			val     string
			wantErr bool
		}{
			{"is one of", []string{"a", "b", "c"}, "b", false},
			{"is not one of", []string{"a", "b", "c"}, "d", true},
			{"not one of with empty allowed", []string{}, "a", true},
			{"one of with empty value", []string{"a", ""}, "", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := OneOf[string](tt.allowed)()
				if err := v(tt.val); (err != nil) != tt.wantErr {
					t.Errorf("OneOf() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("int", func(t *testing.T) {
		tests := []struct {
			name    string
			allowed []int
			val     int
			wantErr bool
		}{
			{"is one of", []int{1, 2, 3}, 2, false},
			{"is not one of", []int{1, 2, 3}, 4, true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := OneOf[int](tt.allowed)()
				if err := v(tt.val); (err != nil) != tt.wantErr {
					t.Errorf("OneOf() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("bool", func(t *testing.T) {
		tests := []struct {
			name    string
			allowed []bool
			val     bool
			wantErr bool
		}{
			{"is one of", []bool{true}, true, false},
			{"is not one of", []bool{true}, false, true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := OneOf[bool](tt.allowed)()
				if err := v(tt.val); (err != nil) != tt.wantErr {
					t.Errorf("OneOf() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestGt(t *testing.T) {
	t.Run("int8", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   int8
			value   int8
			wantErr bool
		}{
			{"greater", int8(5), int8(6), false},
			{"equal", int8(5), int8(5), true},
			{"less", int8(5), int8(4), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[int8]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("int16", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   int16
			value   int16
			wantErr bool
		}{
			{"greater", int16(500), int16(501), false},
			{"equal", int16(500), int16(500), true},
			{"less", int16(500), int16(499), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[int16]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("int32", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   int32
			value   int32
			wantErr bool
		}{
			{"greater", int32(70000), int32(70001), false},
			{"equal", int32(70000), int32(70000), true},
			{"less", int32(70000), int32(69999), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[int32]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("int64", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   int64
			value   int64
			wantErr bool
		}{
			{"greater", int64(9000000000), int64(9000000001), false},
			{"equal", int64(9000000000), int64(9000000000), true},
			{"less", int64(9000000000), int64(8999999999), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[int64]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("float32", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   float32
			value   float32
			wantErr bool
		}{
			{"greater", float32(5.5), float32(5.6), false},
			{"equal", float32(5.5), float32(5.5), true},
			{"less", float32(5.5), float32(5.4), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[float32]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("float64", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   float64
			value   float64
			wantErr bool
		}{
			{"greater", float64(123.45), float64(123.46), false},
			{"equal", float64(123.45), float64(123.45), true},
			{"less", float64(123.45), float64(123.44), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[float64]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("uint", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   uint
			value   uint
			wantErr bool
		}{
			{"greater", uint(5), uint(6), false},
			{"equal", uint(5), uint(5), true},
			{"less", uint(5), uint(4), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[uint]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("uint8", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   uint8
			value   uint8
			wantErr bool
		}{
			{"greater", uint8(5), uint8(6), false},
			{"equal", uint8(5), uint8(5), true},
			{"less", uint8(5), uint8(4), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[uint8]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("uint16", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   uint16
			value   uint16
			wantErr bool
		}{
			{"greater", uint16(500), uint16(501), false},
			{"equal", uint16(500), uint16(500), true},
			{"less", uint16(500), uint16(499), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[uint16]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("uint32", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   uint32
			value   uint32
			wantErr bool
		}{
			{"greater", uint32(70000), uint32(70001), false},
			{"equal", uint32(70000), uint32(70000), true},
			{"less", uint32(70000), uint32(69999), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[uint32]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("uint64", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   uint64
			value   uint64
			wantErr bool
		}{
			{"greater", uint64(9000000000), uint64(9000000001), false},
			{"equal", uint64(9000000000), uint64(9000000000), true},
			{"less", uint64(9000000000), uint64(8999999999), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[uint64]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("time.Time", func(t *testing.T) {
		now := time.Now()
		tests := []struct {
			name    string
			limit   time.Time
			value   time.Time
			wantErr bool
		}{
			{"greater", now, now.Add(time.Minute), false},
			{"equal", now, now, true},
			{"less", now, now.Add(-time.Minute), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gt[time.Time]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestGte(t *testing.T) {
	t.Run("int8", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   int8
			value   int8
			wantErr bool
		}{
			{"value is greater", 5, 6, false},
			{"value is equal", 5, 5, false},
			{"value is less", 5, 4, true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gte(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gte() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("int64", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   int64
			value   int64
			wantErr bool
		}{
			{"value is greater", 5, 6, false},
			{"value is equal", 5, 5, false},
			{"value is less", 5, 4, true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gte(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gte() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("float64", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   float64
			value   float64
			wantErr bool
		}{
			{"value is greater", 5.5, 5.6, false},
			{"value is equal", 5.5, 5.5, false},
			{"value is less", 5.5, 5.4, true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gte(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gte() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("time.Time", func(t *testing.T) {
		now := time.Now()
		tests := []struct {
			name    string
			limit   time.Time
			value   time.Time
			wantErr bool
		}{
			{"value is greater", now, now.Add(time.Minute), false},
			{"value is equal", now, now, false},
			{"value is less", now, now.Add(-time.Minute), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Gte(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Gte() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestLt(t *testing.T) {
	t.Run("int8", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   int8
			value   int8
			wantErr bool
		}{
			{"less", int8(6), int8(5), false},
			{"equal", int8(5), int8(5), true},
			{"greater", int8(4), int8(5), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[int8]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("int16", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   int16
			value   int16
			wantErr bool
		}{
			{"less", int16(501), int16(500), false},
			{"equal", int16(500), int16(500), true},
			{"greater", int16(499), int16(500), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[int16]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("int32", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   int32
			value   int32
			wantErr bool
		}{
			{"less", int32(70001), int32(70000), false},
			{"equal", int32(70000), int32(70000), true},
			{"greater", int32(69999), int32(70000), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[int32]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("int64", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   int64
			value   int64
			wantErr bool
		}{
			{"less", int64(9000000001), int64(9000000000), false},
			{"equal", int64(9000000000), int64(9000000000), true},
			{"greater", int64(8999999999), int64(9000000000), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[int64]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("float32", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   float32
			value   float32
			wantErr bool
		}{
			{"less", float32(5.6), float32(5.5), false},
			{"equal", float32(5.5), float32(5.5), true},
			{"greater", float32(5.4), float32(5.5), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[float32]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("uint", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   uint
			value   uint
			wantErr bool
		}{
			{"less", uint(6), uint(5), false},
			{"equal", uint(5), uint(5), true},
			{"greater", uint(4), uint(5), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[uint]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("uint8", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   uint8
			value   uint8
			wantErr bool
		}{
			{"less", uint8(6), uint8(5), false},
			{"equal", uint8(5), uint8(5), true},
			{"greater", uint8(4), uint8(5), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[uint8]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("uint16", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   uint16
			value   uint16
			wantErr bool
		}{
			{"less", uint16(501), uint16(500), false},
			{"equal", uint16(500), uint16(500), true},
			{"greater", uint16(499), uint16(500), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[uint16]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("uint32", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   uint32
			value   uint32
			wantErr bool
		}{
			{"less", uint32(70001), uint32(70000), false},
			{"equal", uint32(70000), uint32(70000), true},
			{"greater", uint32(69999), uint32(70000), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[uint32]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("uint64", func(t *testing.T) {
		tests := []struct {
			name    string
			limit   uint64
			value   uint64
			wantErr bool
		}{
			{"less", uint64(9000000001), uint64(9000000000), false},
			{"equal", uint64(9000000000), uint64(9000000000), true},
			{"greater", uint64(8999999999), uint64(9000000000), true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[uint64]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("time.Time", func(t *testing.T) {
		now := time.Now()
		tests := []struct {
			name    string
			limit   time.Time
			value   time.Time
			wantErr bool
		}{
			{"less", now, now.Add(-time.Minute), false},
			{"equal", now, now, true},
			{"greater", now.Add(-time.Minute), now, true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, v := Lt(tt.limit)()
				if err := v(tt.value); (err != nil) != tt.wantErr {
					t.Errorf("Lt[time.Time]() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestLte(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		_, v := Lte(5)()
		if err := v(4); err != nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, false)
		}
		if err := v(5); err != nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, false)
		}
		if err := v(6); err == nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("int8", func(t *testing.T) {
		_, v := Lte(int8(5))()
		if err := v(int8(5)); err != nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, false)
		}
		if err := v(int8(6)); err == nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("float32", func(t *testing.T) {
		_, v := Lte(float32(5.5))()
		if err := v(float32(5.5)); err != nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, false)
		}
		if err := v(float32(5.6)); err == nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("time.Time", func(t *testing.T) {
		now := time.Now()
		_, v := Lte(now)()
		if err := v(now.Add(-time.Minute)); err != nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, false)
		}
		if err := v(now); err != nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, false)
		}
		if err := v(now.Add(time.Minute)); err == nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, true)
		}
	})
}

func TestBetween(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		_, v := Between(5, 10)()
		if err := v(4); err == nil {
			t.Errorf("Between() error = nil, wantErr true")
		}
		if err := v(11); err == nil {
			t.Errorf("Between() error = nil, wantErr true")
		}
		if err := v(7); err != nil {
			t.Errorf("Between() error = %v, wantErr nil", err)
		}
		if err := v(5); err != nil {
			t.Errorf("Between() error = %v, wantErr nil", err)
		}
		if err := v(10); err != nil {
			t.Errorf("Between() error = %v, wantErr nil", err)
		}
	})

	t.Run("int64", func(t *testing.T) {
		_, v := Between(int64(5), int64(10))()
		if err := v(int64(4)); err == nil {
			t.Errorf("Between() error = nil, wantErr true")
		}
		if err := v(int64(11)); err == nil {
			t.Errorf("Between() error = nil, wantErr true")
		}
		if err := v(int64(7)); err != nil {
			t.Errorf("Between() error = %v, wantErr nil", err)
		}
	})

	t.Run("float32", func(t *testing.T) {
		_, v := Between(float32(5.5), float32(10.5))()
		if err := v(float32(5.4)); err == nil {
			t.Errorf("Between() error = nil, wantErr true")
		}
		if err := v(float32(10.6)); err == nil {
			t.Errorf("Between() error = nil, wantErr true")
		}
		if err := v(float32(7.5)); err != nil {
			t.Errorf("Between() error = %v, wantErr nil", err)
		}
	})

	t.Run("time.Time", func(t *testing.T) {
		now := time.Now()
		minV := now
		maxV := now.Add(time.Hour)
		_, v := Between(minV, maxV)()
		if err := v(now.Add(-time.Minute)); err == nil {
			t.Errorf("Between() error = nil, wantErr true")
		}
		if err := v(maxV.Add(time.Minute)); err == nil {
			t.Errorf("Between() error = nil, wantErr true")
		}
		if err := v(now.Add(30 * time.Minute)); err != nil {
			t.Errorf("Between() error = %v, wantErr nil", err)
		}
		if err := v(minV); err != nil {
			t.Errorf("Between() error = %v, wantErr nil", err)
		}
		if err := v(maxV); err != nil {
			t.Errorf("Between() error = %v, wantErr nil", err)
		}
	})
}

func TestBeTrue(t *testing.T) {
	tests := []struct {
		name    string
		val     bool
		wantErr bool
	}{
		{"is true", true, false},
		{"is false", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := BeTrue()()
			if err := v(tt.val); (err != nil) != tt.wantErr {
				t.Errorf("BeTrue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBeFalse(t *testing.T) {
	tests := []struct {
		name    string
		val     bool
		wantErr bool
	}{
		{"is false", false, false},
		{"is true", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := BeFalse()()
			if err := v(tt.val); (err != nil) != tt.wantErr {
				t.Errorf("BeFalse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
