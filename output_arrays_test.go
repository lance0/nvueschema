package nvueschema

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestObjectArrayModels(t *testing.T) {
	entry := &Config{Type: "object", Properties: map[string]*Config{"name": {Type: "string"}, "children": {Type: "array", Items: &Config{Properties: map[string]*Config{"id": {Type: "integer"}}}}}}
	schema := &Config{Properties: map[string]*Config{"entries": {Type: "array", Items: entry}, "groups": {AdditionalProperties: &Config{Properties: map[string]*Config{"entries": {Type: "array", Items: entry}}}}}}
	t.Run("go", func(t *testing.T) {
		var b bytes.Buffer
		if err := WriteGoStructs(&b, schema, nil); err != nil {
			t.Fatal(err)
		}
		compileGo(t, b.Bytes())
	})
	t.Run("python", func(t *testing.T) {
		var b bytes.Buffer
		if err := WritePydantic(&b, schema, nil); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "class NvueConfigEntriesItem(") {
			t.Error("array item model missing")
		}
		t.Run("validate", func(t *testing.T) {
			runPydantic(t, b.Bytes(), `
config = {"entries": [{"name": "a", "children": [{"id": 1}]}], "groups": {"one": {"entries": [{"name": "b"}]}}}
model = models.NvueConfig.model_validate(config)
assert model.model_dump(by_alias=True, exclude_none=True) == config
models.NvueConfig.model_json_schema()
`)
		})
	})
	t.Run("protobuf", func(t *testing.T) {
		var b bytes.Buffer
		if err := WriteProtobuf(&b, schema, nil, false); err != nil {
			t.Fatal(err)
		}
		compileProto(t, b.Bytes())
	})
}

func TestGoGeneratedImports(t *testing.T) {
	for _, format := range []string{"", "ipv4"} {
		t.Run(format, func(t *testing.T) {
			var b bytes.Buffer
			if err := WriteGoStructs(&b, &Config{Properties: map[string]*Config{"value": {Type: "string", Format: format}}}, nil); err != nil {
				t.Fatal(err)
			}
			compileGo(t, b.Bytes())
		})
	}
}

func compileGo(t *testing.T, source []byte) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "config.go")
	if err := os.WriteFile(file, source, 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "test", file).CombinedOutput(); err != nil {
		t.Fatalf("Go compile: %v\n%s", err, out)
	}
}
