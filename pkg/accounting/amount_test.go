package accounting

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestParseDecimalToRawExact(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		raw, err := ParseDecimalToRawExact("1.23", 2, false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !raw.Equal(decimal.NewFromInt(123)) {
			t.Fatalf("expected 123, got %s", raw.String())
		}
	})

	t.Run("reject_scientific", func(t *testing.T) {
		_, err := ParseDecimalToRawExact("1e3", 0, false)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("reject_implicit_rounding", func(t *testing.T) {
		_, err := ParseDecimalToRawExact("0.0000001", 6, true)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("zero_handling", func(t *testing.T) {
		if _, err := ParseDecimalToRawExact("0", 6, false); err == nil {
			t.Fatalf("expected error")
		}
		raw, err := ParseDecimalToRawExact("0", 6, true)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !raw.Equal(decimal.Zero) {
			t.Fatalf("expected 0, got %s", raw.String())
		}
	})
}

func TestValidateDecimal65Int(t *testing.T) {
	ok := strings.Repeat("9", 65)
	okDec, _ := decimal.NewFromString(ok)
	if err := ValidateDecimal65Int(okDec); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}

	tooBig := strings.Repeat("9", 66)
	tooBigDec, _ := decimal.NewFromString(tooBig)
	if err := ValidateDecimal65Int(tooBigDec); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRawToDecimalString(t *testing.T) {
	raw := decimal.NewFromInt(123)
	s, err := RawToDecimalString(raw, 2)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if s != "1.23" {
		t.Fatalf("expected 1.23, got %s", s)
	}
}
