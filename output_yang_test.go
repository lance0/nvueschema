package nvueschema

import (
	"bytes"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestYANGTypeStatements(t *testing.T) {
	var buf bytes.Buffer
	schema := &Config{Properties: map[string]*Config{"label": {Type: "string"}}}
	if err := WriteYANG(&buf, schema, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "};") {
		t.Error("a YANG block must not end with a semicolon")
	}
	t.Run("parse", func(t *testing.T) {
		parseYANG(t, buf.Bytes())
	})
}

func TestYANGPatternStrings(t *testing.T) {
	tests := []struct {
		name, pattern, literal string
	}{
		{"apostrophe", `[a-z']+`, `"[a-z']+"`},
		{"both-quotes", `[a-z'"]+`, `"[a-z'\"]+"`},
		{"backslashes", `\d+\.[a-z']+`, `"\\d+\\.[a-z']+"`},
		{"newline", "first \n  second'", `"first \n  second'"`},
		{"tab", "left\t'right", `"left\t'right"`},
		{"unicode", `[é中']+`, `"[é中']+"`},
	}
	schema := &Config{Properties: make(map[string]*Config)}
	for _, tt := range tests {
		schema.Properties[tt.name] = &Config{Type: "string", Pattern: tt.pattern}
	}
	var buf bytes.Buffer
	if err := WriteYANG(&buf, schema, nil); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		if !strings.Contains(buf.String(), "pattern "+tt.literal+";") {
			t.Errorf("%s: missing correctly quoted pattern %s", tt.name, tt.literal)
		}
	}

	t.Run("parse", func(t *testing.T) {
		// YIN exposes the parsed string value, so this verifies that quoting
		// preserves the regex rather than merely producing valid YANG syntax.
		var module struct {
			Leaves []struct {
				Name    string `xml:"name,attr"`
				Pattern struct {
					Value string `xml:"value,attr"`
				} `xml:"type>pattern"`
			} `xml:"container>leaf"`
		}
		if err := xml.Unmarshal(parseYANG(t, buf.Bytes()), &module); err != nil {
			t.Fatal(err)
		}
		patterns := make(map[string]string)
		for _, leaf := range module.Leaves {
			patterns[leaf.Name] = leaf.Pattern.Value
		}
		for _, tt := range tests {
			if got, ok := patterns[tt.name]; !ok || got != tt.pattern {
				t.Errorf("%s: parsed pattern = %q, want %q", tt.name, got, tt.pattern)
			}
		}
	})
}

func parseYANG(t *testing.T, source []byte) []byte {
	t.Helper()
	python := testPython(t, "pyang")
	file := filepath.Join(t.TempDir(), "cumulus-nvue.yang")
	if err := os.WriteFile(file, source, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "-m", "pyang", "-f", "yin", file)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pyang: %v\n%s", err, &stderr)
	}
	return out
}

// Set NVUESCHEMA_PYTHON to a Python environment with pyang and pydantic to
// exercise the generated artifacts in addition to the Go-only tests.
func testPython(t *testing.T, module string) string {
	t.Helper()
	python := os.Getenv("NVUESCHEMA_PYTHON")
	if python == "" {
		var err error
		python, err = exec.LookPath("python3")
		if err != nil {
			t.Skip("install Python for generator integration tests")
		}
	}
	if out, err := exec.Command(python, "-c", "import "+module).CombinedOutput(); err != nil {
		if os.Getenv("NVUESCHEMA_PYTHON") != "" {
			t.Fatalf("configured Python lacks %s: %v\n%s", module, err, out)
		}
		t.Skipf("install %s for generator integration tests", module)
	}
	return python
}
