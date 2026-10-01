package nvueschema

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPydanticConstraints(t *testing.T) {
	min, max := 552.0, 9216.0
	lo, hi := 2, 8
	large := float64(1 << 64)
	schema := &Config{Properties: map[string]*Config{
		"counter": {Type: "integer", Maximum: &large},
		"mtu":     {Type: "integer", Minimum: &min, Maximum: &max},
		"name":    {Type: "string", MinLength: &lo, MaxLength: &hi, Pattern: `^(?!none$)[a-z]+$`},
		"timer":   {AnyOf: []*Config{{Type: "integer", Minimum: &min, Maximum: &max}, {Type: "string", Enum: []any{"auto"}}}},
		"mac":     {Type: "string", Format: "mac", Pattern: `^00:`},
	}}
	var buf bytes.Buffer
	if err := WritePydantic(&buf, schema, nil); err != nil {
		t.Fatal(err)
	}
	// These checks also run when the Python integration dependencies are absent.
	if !bytes.Contains(buf.Bytes(), []byte("ge=552")) || !bytes.Contains(buf.Bytes(), []byte("le=9216")) {
		t.Error("numeric bounds missing")
	}
	t.Run("validate", func(t *testing.T) {
		runPydantic(t, buf.Bytes(), `
from pydantic import ValidationError
for value in ({"counter": 2**64}, {"mtu": 552}, {"mtu": 9216}, {"name": "ok"}, {"timer": "auto"}, {"timer": 552}, {"mac": "00:11:22:33:44:55"}):
    models.NvueConfig.model_validate(value)
for value in ({"counter": 2**64+1}, {"mtu": -1}, {"mtu": 9217}, {"name": "a"}, {"name": "toolongname"}, {"name": "none"}, {"name": "ABC"}, {"timer": 1}, {"timer": "bad"}, {"mac": "11:11:22:33:44:55"}, {"mac": "00:bad"}):
    try:
        models.NvueConfig.model_validate(value)
    except ValidationError:
        pass
    else:
        raise AssertionError(f"accepted invalid value: {value}")
`)
	})
}

func runPydantic(t *testing.T, source []byte, assertions string) {
	t.Helper()
	python := testPython(t, "pydantic")
	dir := t.TempDir()
	file := filepath.Join(dir, "models.py")
	if err := os.WriteFile(file, source, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "-c", "import models\n"+assertions)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Python: %v\n%s", err, out)
	}
}
