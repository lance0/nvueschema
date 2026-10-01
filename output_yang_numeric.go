package nvueschema

import (
	"fmt"
	"math"
	"strings"
)

// Use micro-unit precision by default, increasing it for explicit fractional
// values or reducing it to accommodate large bounds. Refuse incompatible
// precision and magnitude instead of rounding declared values.
func yangDecimalPrecision(s *Config) (int, error) {
	var values []float64
	for _, bound := range []*float64{s.Minimum, s.Maximum} {
		if bound != nil {
			values = append(values, *bound)
		}
	}
	if value, ok := s.Default.(float64); ok {
		values = append(values, value)
	}
	for _, value := range s.Enum {
		if number, ok := value.(float64); ok {
			values = append(values, number)
		}
	}
	required, magnitude := 1, 0.0
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, fmt.Errorf("decimal64 cannot represent %v", value)
		}
		if _, fraction, ok := strings.Cut(yangNumber(value), "."); ok {
			required = max(required, len(fraction))
		}
		magnitude = max(magnitude, math.Abs(value))
	}
	digits := max(6, required)
	for digits > required && magnitude >= math.Ldexp(1, 63)/math.Pow10(digits) {
		digits--
	}
	if digits > 18 || magnitude >= math.Ldexp(1, 63)/math.Pow10(digits) {
		return 0, fmt.Errorf("decimal64 cannot represent magnitude %g with %d fraction digits", magnitude, required)
	}
	return digits, nil
}
