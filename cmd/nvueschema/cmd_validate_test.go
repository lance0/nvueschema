package main

import (
	"os"
	"os/exec"
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
			cmd := exec.Command(os.Args[0], "-test.run=^TestValidateCommandHelper$", "--", spec, config)
			cmd.Env = append(os.Environ(), "NVUESCHEMA_VALIDATE_HELPER=1")
			output, err := cmd.CombinedOutput()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v\n%s", tc.valid, err, output)
			}
		})
	}
}

// Exercise the command's exit status without changing its production behavior.
func TestValidateCommandHelper(t *testing.T) {
	if os.Getenv("NVUESCHEMA_VALIDATE_HELPER") != "1" {
		return
	}
	cmd := newValidateCmd()
	cmd.SetArgs(os.Args[len(os.Args)-2:])
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
