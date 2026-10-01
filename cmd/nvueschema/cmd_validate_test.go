package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateNVUEPattern(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.json")
	if err := os.WriteFile(spec, []byte(`{"x-defs":{"cue-patch-schema-root-root":{"properties":{"profile":{"anyOf":[{"type":"string","pattern":"^(?!none$).*$"}]}}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, config string
		valid        bool
	}{
		{"ordinary", `{"profile":"fast"}`, true},
		{"prefix", `{"profile":"none-fast"}`, true},
		{"empty", `{"profile":""}`, true},
		{"reserved", `{"profile":"none"}`, false},
		{"newline", `{"profile":"fast\nslow"}`, false},
		{"wrong type", `{"profile":42}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := filepath.Join(dir, "config.json")
			if err := os.WriteFile(config, []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := newValidateCmd()
			cmd.SetArgs([]string{spec, config})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			err := cmd.Execute()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
