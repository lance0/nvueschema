package nvueschema

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtobufMessageScopes(t *testing.T) {
	settings := &Config{Properties: map[string]*Config{"value": {Type: "string"}}}
	schema := &Config{Properties: map[string]*Config{
		"first":       {Properties: map[string]*Config{"settings": settings}},
		"second":      {Properties: map[string]*Config{"settings": settings}},
		"ports":       {AdditionalProperties: settings},
		"ports-value": settings,
		"ports-entry": settings,
	}}
	var buf bytes.Buffer
	if err := WriteProtobuf(&buf, schema, nil, false); err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(buf.String(), "message Settings {"); count != 2 {
		t.Errorf("got %d Settings definitions, want two independent scopes", count)
	}
	t.Run("compile", func(t *testing.T) { compileProto(t, buf.Bytes()) })
}

func compileProto(t *testing.T, data []byte) {
	t.Helper()
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("install protoc to compile generated Protobuf")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "config.proto")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(protoc, "-I", dir, "--descriptor_set_out="+filepath.Join(dir, "config.pb"), file).CombinedOutput(); err != nil {
		t.Fatalf("protoc: %v\n%s", err, out)
	}
}
