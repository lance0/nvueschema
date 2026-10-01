package nvueschema

import (
	"bytes"
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
		python := testPython(t, "pyang")
		dir := t.TempDir()
		file := filepath.Join(dir, "cumulus-nvue.yang")
		if err := os.WriteFile(file, buf.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(python, "-m", "pyang", file).CombinedOutput(); err != nil {
			t.Fatalf("pyang: %v\n%s", err, out)
		}
	})
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
