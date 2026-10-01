package nvueschema

import "testing"

func TestJSONSchemaPreservesPatterns(t *testing.T) {
	for _, pattern := range []string{`^(?!none$).*$`, `^(?=.*[0-9])[a-z0-9]+$`} {
		scalar := &Config{Type: "string", Pattern: pattern}
		for _, schema := range []*Config{scalar, {AnyOf: []*Config{scalar}}} {
			got := schema.ToJSONSchema()
			if got["pattern"] != pattern {
				t.Errorf("pattern=%v, want %q", got["pattern"], pattern)
			}
		}
	}
}
