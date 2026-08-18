// Package money represents monetary values exactly as currency-qualified minor
// units. Decimal text is accepted at the boundary; floating point is never.
package money

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
)

var (
	ErrUnsupportedCurrency = errors.New("unsupported currency")
	ErrInvalidAmount       = errors.New("invalid monetary amount")
	ErrCurrencyMismatch    = errors.New("currency mismatch")
	ErrInsufficientAmount  = errors.New("insufficient monetary amount")
)

// Currency defines the decimal exponent used for one supported ISO 4217 code.
type Currency struct {
	Code     string
	Exponent uint8
}

var supported = map[string]Currency{
	"EUR": {Code: "EUR", Exponent: 2},
	"GBP": {Code: "GBP", Exponent: 2},
	"INR": {Code: "INR", Exponent: 2},
	"JPY": {Code: "JPY", Exponent: 0},
	"KWD": {Code: "KWD", Exponent: 3},
	"USD": {Code: "USD", Exponent: 2},
}

// LookupCurrency returns the registered exponent for a currency code.
func LookupCurrency(code string) (Currency, error) {
	currency, ok := supported[strings.ToUpper(strings.TrimSpace(code))]
	if !ok {
		return Currency{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, code)
	}
	return currency, nil
}

// Money is a non-negative value in its currency's smallest supported unit.
// Minor is intentionally private so callers cannot create invalid currencies.
type Money struct {
	minor    int64
	currency Currency
}

// New constructs an exact value from a non-negative minor-unit integer.
func New(currencyCode string, minor int64) (Money, error) {
	if minor < 0 {
		return Money{}, fmt.Errorf("%w: negative minor units", ErrInvalidAmount)
	}
	currency, err := LookupCurrency(currencyCode)
	if err != nil {
		return Money{}, err
	}
	return Money{minor: minor, currency: currency}, nil
}

// Parse converts canonical decimal text to exact minor units. Signs, exponent
// notation, whitespace within a number, excessive precision, and overflow are
// rejected rather than rounded.
func Parse(currencyCode, decimal string) (Money, error) {
	currency, err := LookupCurrency(currencyCode)
	if err != nil {
		return Money{}, err
	}
	value := strings.TrimSpace(decimal)
	if value == "" || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return Money{}, fmt.Errorf("%w: expected unsigned decimal", ErrInvalidAmount)
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return Money{}, fmt.Errorf("%w: malformed decimal", ErrInvalidAmount)
	}
	for _, digit := range parts[0] {
		if !unicode.IsDigit(digit) || digit > '9' {
			return Money{}, fmt.Errorf("%w: non-ASCII digit", ErrInvalidAmount)
		}
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > int(currency.Exponent) {
		return Money{}, fmt.Errorf("%w: precision exceeds %s exponent", ErrInvalidAmount, currency.Code)
	}
	for _, digit := range fraction {
		if digit < '0' || digit > '9' {
			return Money{}, fmt.Errorf("%w: non-ASCII digit", ErrInvalidAmount)
		}
	}
	if len(parts) == 2 && fraction == "" {
		return Money{}, fmt.Errorf("%w: missing fractional digits", ErrInvalidAmount)
	}

	digits := parts[0] + fraction + strings.Repeat("0", int(currency.Exponent)-len(fraction))
	minor, err := parseMinor(digits)
	if err != nil {
		return Money{}, err
	}
	return Money{minor: minor, currency: currency}, nil
}

func parseMinor(digits string) (int64, error) {
	var minor int64
	for _, digit := range digits {
		if minor > (math.MaxInt64-int64(digit-'0'))/10 {
			return 0, fmt.Errorf("%w: amount overflows minor-unit range", ErrInvalidAmount)
		}
		minor = minor*10 + int64(digit-'0')
	}
	return minor, nil
}

func (m Money) Minor() int64       { return m.minor }
func (m Money) Currency() Currency { return m.currency }
func (m Money) IsZero() bool       { return m.minor == 0 }
func (m Money) IsPositive() bool   { return m.minor > 0 }
func (m Money) String() string {
	scale := pow10(m.currency.Exponent)
	whole := m.minor / scale
	if m.currency.Exponent == 0 {
		return fmt.Sprintf("%s %d", m.currency.Code, whole)
	}
	return fmt.Sprintf("%s %d.%0*d", m.currency.Code, whole, m.currency.Exponent, m.minor%scale)
}

// Add combines values only when their currencies agree and the result fits.
func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, ErrCurrencyMismatch
	}
	if m.minor > math.MaxInt64-other.minor {
		return Money{}, fmt.Errorf("%w: amount overflows minor-unit range", ErrInvalidAmount)
	}
	return Money{minor: m.minor + other.minor, currency: m.currency}, nil
}

// Subtract refuses a negative result; account projections may never underflow.
func (m Money) Subtract(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, ErrCurrencyMismatch
	}
	if m.minor < other.minor {
		return Money{}, ErrInsufficientAmount
	}
	return Money{minor: m.minor - other.minor, currency: m.currency}, nil
}

func pow10(exponent uint8) int64 {
	scale := int64(1)
	for range exponent {
		scale *= 10
	}
	return scale
}
