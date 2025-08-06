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
		{"match", "a*c", "abc", false},
		{"match", "a*c", "adefbc", false},
		{"no match", "a*c", "abd", true},
		{"match with ?", "a?c", "abc", false},
		{"no match with ?", "a?c", "ac", true},
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

func TestEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"invalid email", "test", true},
		{"invalid email", "test@", true},
		{"invalid email", "@example.com", true},
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
		{"invalid url01", "example.com", true},
		{"invalid url02", "http://", true},
		{"malformed url", "://a.b", true},
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
	t.Run("int", func(t *testing.T) {
		_, v := Gt(5)()
		if err := v(6); err != nil {
			t.Errorf("Gt() error = %v, wantErr %v", err, false)
		}
		if err := v(5); err == nil {
			t.Errorf("Gt() error = %v, wantErr %v", err, true)
		}
		if err := v(4); err == nil {
			t.Errorf("Gt() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("float64", func(t *testing.T) {
		_, v := Gt(5.5)()
		if err := v(5.6); err != nil {
			t.Errorf("Gt() error = %v, wantErr %v", err, false)
		}
		if err := v(5.5); err == nil {
			t.Errorf("Gt() error = %v, wantErr %v", err, true)
		}
		if err := v(5.4); err == nil {
			t.Errorf("Gt() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("time.Time", func(t *testing.T) {
		now := time.Now()
		_, v := Gt(now)()
		if err := v(now.Add(time.Minute)); err != nil {
			t.Errorf("Gt() error = %v, wantErr %v", err, false)
		}
		if err := v(now); err == nil {
			t.Errorf("Gt() error = %v, wantErr %v", err, true)
		}
		if err := v(now.Add(-time.Minute)); err == nil {
			t.Errorf("Gt() error = %v, wantErr %v", err, true)
		}
	})
}

func TestGte(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		_, v := Gte(5)()
		if err := v(6); err != nil {
			t.Errorf("Gte() error = %v, wantErr %v", err, false)
		}
		if err := v(5); err != nil {
			t.Errorf("Gte() error = %v, wantErr %v", err, false)
		}
		if err := v(4); err == nil {
			t.Errorf("Gte() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("float64", func(t *testing.T) {
		_, v := Gte(5.5)()
		if err := v(5.6); err != nil {
			t.Errorf("Gte() error = %v, wantErr %v", err, false)
		}
		if err := v(5.5); err != nil {
			t.Errorf("Gte() error = %v, wantErr %v", err, false)
		}
		if err := v(5.4); err == nil {
			t.Errorf("Gte() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("time.Time", func(t *testing.T) {
		now := time.Now()
		_, v := Gte(now)()
		if err := v(now.Add(time.Minute)); err != nil {
			t.Errorf("Gte() error = %v, wantErr %v", err, false)
		}
		if err := v(now); err != nil {
			t.Errorf("Gte() error = %v, wantErr %v", err, false)
		}
		if err := v(now.Add(-time.Minute)); err == nil {
			t.Errorf("Gte() error = %v, wantErr %v", err, true)
		}
	})
}

func TestLt(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		_, v := Lt(5)()
		if err := v(4); err != nil {
			t.Errorf("Lt() error = %v, wantErr %v", err, false)
		}
		if err := v(5); err == nil {
			t.Errorf("Lt() error = %v, wantErr %v", err, true)
		}
		if err := v(6); err == nil {
			t.Errorf("Lt() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("float64", func(t *testing.T) {
		_, v := Lt(5.5)()
		if err := v(5.4); err != nil {
			t.Errorf("Lt() error = %v, wantErr %v", err, false)
		}
		if err := v(5.5); err == nil {
			t.Errorf("Lt() error = %v, wantErr %v", err, true)
		}
		if err := v(5.6); err == nil {
			t.Errorf("Lt() error = %v, wantErr %v", err, true)
		}
	})

	t.Run("time.Time", func(t *testing.T) {
		now := time.Now()
		_, v := Lt(now)()
		if err := v(now.Add(-time.Minute)); err != nil {
			t.Errorf("Lt() error = %v, wantErr %v", err, false)
		}
		if err := v(now); err == nil {
			t.Errorf("Lt() error = %v, wantErr %v", err, true)
		}
		if err := v(now.Add(time.Minute)); err == nil {
			t.Errorf("Lt() error = %v, wantErr %v", err, true)
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

	t.Run("float64", func(t *testing.T) {
		_, v := Lte(5.5)()
		if err := v(5.4); err != nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, false)
		}
		if err := v(5.5); err != nil {
			t.Errorf("Lte() error = %v, wantErr %v", err, false)
		}
		if err := v(5.6); err == nil {
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

	t.Run("float64", func(t *testing.T) {
		_, v := Between(5.5, 10.5)()
		if err := v(5.4); err == nil {
			t.Errorf("Between() error = nil, wantErr true")
		}
		if err := v(10.6); err == nil {
			t.Errorf("Between() error = nil, wantErr true")
		}
		if err := v(7.5); err != nil {
			t.Errorf("Between() error = %v, wantErr nil", err)
		}
		if err := v(5.5); err != nil {
			t.Errorf("Between() error = %v, wantErr nil", err)
		}
		if err := v(10.5); err != nil {
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
