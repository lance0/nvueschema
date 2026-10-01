package main

import (
	"github.com/santhosh-tekuri/jsonschema/v6"
	"testing"
)

func TestValidationPatterns(t *testing.T) {
	for _, tc := range []struct{ name, pattern, good, bad string }{
		{"reserved alternatives", `^(?!(?:none|auto)$).*$`, "normal", "auto"},
		{"positive lookahead", `^(?=.*[0-9])[a-z0-9]+$`, "port1", "port"},
		{"backreference", `^([a-z]+)-\1$`, "same-same", "same-other"},
		{"lookbehind", `(?<=prefix-)value$`, "prefix-value", "other-value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compiler := jsonschema.NewCompiler()
			compiler.UseRegexpEngine(compilePattern)
			// Include a nested schema, so this verifies the compiler actually uses
			// the configured engine rather than just testing the regex adapter.
			doc := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string", "pattern": tc.pattern}}}
			if err := compiler.AddResource("schema.json", doc); err != nil {
				t.Fatal(err)
			}
			schema, err := compiler.Compile("schema.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(map[string]any{"value": tc.good}); err != nil {
				t.Errorf("valid value: %v", err)
			}
			if err := schema.Validate(map[string]any{"value": tc.bad}); err == nil {
				t.Error("accepted invalid value")
			}
		})
	}
	if _, err := compilePattern(`(?`); err == nil {
		t.Error("accepted invalid regex")
	}
}
