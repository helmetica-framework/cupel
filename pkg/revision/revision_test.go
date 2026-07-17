package revision

import (
	"testing"
)

// toInt coerces a YAML-decoded number (int/int64/float64) to int for comparison.
func toInt(t *testing.T, v any) int {
	t.Helper()
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		t.Fatalf("value is %T, want a number", v)
		return 0
	}
}
