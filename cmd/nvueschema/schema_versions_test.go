package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// TestSchemaVersions checks downloaded 5.x schemas without making the normal
// test suite depend on network access. Once enabled, all validators are required.
func TestSchemaVersions(t *testing.T) {
	specDir := os.Getenv("NVUESCHEMA_SPEC_DIR")
	if specDir == "" {
		t.Skip("set NVUESCHEMA_SPEC_DIR to run the NVUE version matrix")
	}
	manifest, err := os.ReadFile(filepath.Join("..", "..", "testdata", "schema-versions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var versions []struct {
		Version string
		SHA256  string
	}
	if err := json.Unmarshal(manifest, &versions); err != nil || len(versions) == 0 {
		t.Fatalf("reading version manifest: %v (%d versions)", err, len(versions))
	}
	python := os.Getenv("NVUESCHEMA_PYTHON")
	if python == "" {
		t.Fatal("NVUESCHEMA_PYTHON is required for the version matrix")
	}
	runSchemaTool(t, "", python, "-c", "import pyang, pydantic")
	for _, tool := range []string{"go", "protoc"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("version matrix requires %s: %v", tool, err)
		}
	}
	protoInclude := os.Getenv("NVUESCHEMA_PROTO_INCLUDE")
	if _, err := os.Stat(filepath.Join(protoInclude, "buf", "validate", "validate.proto")); protoInclude == "" || err != nil {
		t.Fatal("NVUESCHEMA_PROTO_INCLUDE must contain buf/validate/validate.proto")
	}

	for _, version := range versions {
		t.Run(version.Version, func(t *testing.T) {
			file := filepath.Join(specDir, "openapi-"+version.Version+".json")
			t.Run("parse", func(t *testing.T) {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != version.SHA256 {
					t.Fatalf("schema checksum %s differs from manifest %s; verify the downloaded release", got, version.SHA256)
				}
				parser, err := resolveSpec(file, true)
				if err != nil {
					t.Fatal(err)
				}
				schema, err := parser.ConfigSchema()
				if err != nil {
					t.Fatal(err)
				}
				if schema == nil || len(parser.ConfigPaths()) == 0 {
					t.Fatal("parsed schema has no configurable paths")
				}
			})
			for _, format := range []string{"jsonschema", "openapi", "go", "pydantic", "yang", "protobuf", "protobuf-validate"} {
				t.Run(format, func(t *testing.T) {
					dir := t.TempDir()
					filename := map[string]string{
						"jsonschema": "schema.json", "openapi": "openapi.json",
						"go": "config.go", "pydantic": "models.py", "yang": "cumulus-nvue.yang",
						"protobuf": "config.proto", "protobuf-validate": "config.proto",
					}[format]
					output := filepath.Join(dir, filename)
					generator := strings.TrimSuffix(format, "-validate")
					args := []string{file, "--format", generator, "--output", output}
					if format == "protobuf-validate" {
						args = append(args, "--validate")
					}
					cmd := newGenerateCmd()
					cmd.SetArgs(args)
					cmd.SetOut(&bytes.Buffer{})
					cmd.SetErr(&bytes.Buffer{})
					if err := cmd.Execute(); err != nil {
						t.Fatalf("generating %s: %v", format, err)
					}
					switch format {
					case "jsonschema", "openapi":
						checkVersionJSONSchema(t, output, format == "openapi")
					case "go":
						runSchemaTool(t, dir, "go", "test", output)
					case "pydantic":
						runSchemaTool(t, dir, python, "-c", versionPydanticCheck)
					case "yang":
						runSchemaTool(t, dir, python, "-m", "pyang", output)
					case "protobuf", "protobuf-validate":
						runSchemaTool(t, dir, "protoc", "-I", dir, "-I", protoInclude,
							"--descriptor_set_out="+filepath.Join(dir, "config.pb"), output)
					}
				})
			}
		})
	}
}

func checkVersionJSONSchema(t *testing.T, filename string, openAPI bool) {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseRegexpEngine(compilePattern)
	if err := compiler.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	location := "schema.json"
	if openAPI {
		location += "#/components/schemas/NvueConfig"
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		mtu   any
		valid bool
	}{
		{"valid MTU", 1500, true},
		{"negative MTU", -1, false},
		{"wrong MTU type", true, false},
	} {
		config := map[string]any{"interface": map[string]any{"swp1": map[string]any{"link": map[string]any{"mtu": tc.mtu}}}}
		if err := schema.Validate(config); (err == nil) != tc.valid {
			t.Errorf("%s: validation error = %v, want valid=%t", tc.name, err, tc.valid)
		}
	}
}

func runSchemaTool(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, out)
	}
}

const versionPydanticCheck = `
import models
from pydantic import ValidationError

config = {"interface": {"swp1": {"link": {"mtu": 1500}}}}
model = models.NvueConfig.model_validate(config)
assert model.model_dump(by_alias=True, exclude_none=True)["interface"]["swp1"]["link"]["mtu"] == 1500
models.NvueConfig.model_json_schema()
for mtu in (-1, {}):
    config["interface"]["swp1"]["link"]["mtu"] = mtu
    try:
        models.NvueConfig.model_validate(config)
    except ValidationError:
        pass
    else:
        raise AssertionError(f"accepted invalid MTU {mtu!r}")
`
