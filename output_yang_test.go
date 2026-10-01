package nvueschema

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"math"
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

func TestYANGDefaults(t *testing.T) {
	tests := []struct {
		name   string
		schema *Config
		want   string
	}{
		{"enum", &Config{Type: "string", Enum: []any{"packet", "byte"}, Default: "packet"}, "packet"},
		{"mac", &Config{Type: "string", Format: "mac", Default: "ff:ff:ff:ff:ff:ff"}, "ff:ff:ff:ff:ff:ff"},
		{"quoted", &Config{Type: "string", Default: "a'\"b"}, "a'\"b"},
		{"empty", &Config{Type: "string", Default: ""}, ""},
		{"integer", &Config{Type: "integer", Default: float64(4294967295)}, "4294967295"},
		{"boolean", &Config{Type: "boolean", Default: true}, "true"},
		{"union", &Config{AnyOf: []*Config{{Type: "integer"}, {Type: "string", Enum: []any{"auto"}}}, Default: "auto"}, "auto"},
	}
	schema := &Config{Properties: make(map[string]*Config)}
	for _, tt := range tests {
		schema.Properties[tt.name] = tt.schema
	}
	var buf bytes.Buffer
	if err := WriteYANG(&buf, schema, nil); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		if want := "default " + yangString(tt.want) + ";"; !strings.Contains(buf.String(), want) {
			t.Errorf("%s: missing %s", tt.name, want)
		}
	}
	t.Run("parse", func(t *testing.T) {
		var module struct {
			Leaves []struct {
				Name    string `xml:"name,attr"`
				Default struct {
					Value string `xml:"value,attr"`
				} `xml:"default"`
			} `xml:"container>leaf"`
		}
		if err := xml.Unmarshal(parseYANG(t, buf.Bytes()), &module); err != nil {
			t.Fatal(err)
		}
		defaults := make(map[string]string)
		for _, leaf := range module.Leaves {
			defaults[leaf.Name] = leaf.Default.Value
		}
		for _, tt := range tests {
			if got, ok := defaults[tt.name]; !ok || got != tt.want {
				t.Errorf("%s: parsed default = %q, want %q", tt.name, got, tt.want)
			}
		}
	})
}

func TestYANGIntegerRanges(t *testing.T) {
	zero, lower, upper := 0.0, 96.0, float64(math.MaxUint32)
	signedMin, signedMax, unsignedMax := float64(math.MinInt64), float64(math.MaxInt64), float64(math.MaxUint64)
	schema := &Config{Properties: map[string]*Config{
		"bounded":  {Type: "integer", Minimum: &lower, Maximum: &upper, Default: float64(960)},
		"signed":   {Type: "integer", Minimum: &signedMin, Maximum: &signedMax},
		"unsigned": {Type: "integer", Format: "integer", Minimum: &zero, Maximum: &unsignedMax},
	}}
	var buf bytes.Buffer
	if err := WriteYANG(&buf, schema, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`range "96..4294967295";`, `range "-9223372036854775808..max";`, `range "0..max";`, "type uint64"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %s", want)
		}
	}
	t.Run("validate", func(t *testing.T) {
		checkYANGValues(t, buf.Bytes(), map[string]yangValues{
			"bounded":  {Good: []string{"96", "960", "4294967295"}, Bad: []string{"95", "4294967296"}},
			"signed":   {Good: []string{"-9223372036854775808", "9223372036854775807"}, Bad: []string{"-9223372036854775809", "9223372036854775808"}},
			"unsigned": {Good: []string{"0", "18446744073709551615"}, Bad: []string{"-1", "18446744073709551616"}},
		})
	})
}

type yangValues struct{ Good, Bad []string }

// Exercise pyang's resolved type restrictions with actual leaf values.
func checkYANGValues(t *testing.T, source []byte, values map[string]yangValues) {
	t.Helper()
	python := testPython(t, "pyang")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cumulus-nvue.yang"), source, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "-c", `
import json, sys
from pyang import context, error, repository
ctx = context.Context(repository.FileRepository("."))
with open("cumulus-nvue.yang") as f:
    module = ctx.add_module("cumulus-nvue.yang", f.read())
ctx.validate()
errors = [(str(pos), tag, args) for pos, tag, args in ctx.errors if error.is_error(error.err_level(tag))]
assert not errors, errors
container = module.search_one("container")
for name, cases in json.load(sys.stdin).items():
    leaf = next(s for s in container.substmts if s.keyword == "leaf" and s.arg == name)
    spec = leaf.search_one("type").i_type_spec
    for category, expected in (("Good", True), ("Bad", False)):
        for text in cases[category] or []:
            errors = []
            value = spec.str_to_val(errors, leaf.pos, text, module)
            valid = value is not None and spec.validate(errors, leaf.pos, value, module)
            assert bool(valid) == expected, (name, text, expected, errors)
`)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(data)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("YANG values: %v\n%s", err, out)
	}
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
