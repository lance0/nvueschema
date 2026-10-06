package nvueschema

import (
	"bytes"
	"encoding/json"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"testing"
)

func TestJSONSchemaPreservesPatterns(t *testing.T) {
	for _, pattern := range []string{`^(?!none$).*$`, `^(?=.*[0-9])[a-z0-9]+$`} {
		scalar := &Config{Type: "string", Pattern: pattern}
		for _, schema := range []*Config{scalar, {AnyOf: []*Config{scalar}}} {
			got := schema.ToJSONSchema()
			if got["pattern"] != pattern {
				t.Errorf("pattern=%v, want %q", got["pattern"], pattern)
			}
		}
	}
}

func TestJSONSchemaConstraints(t *testing.T) {
	maxLength := 3
	for _, tc := range []struct {
		name      string
		schema    *Config
		good, bad []any
	}{
		{"formatted bounds", &Config{Type: "string", Format: "key-string", MaxLength: &maxLength, Pattern: `^a`, Default: "abc"}, []any{"abc"}, []any{"abcd", "bbb"}},
		{"formatted enum", &Config{Type: "string", Format: "key-string", Enum: []any{"yes"}}, []any{"yes"}, []any{"no"}},
		{"formatted nullable", &Config{Type: "string", Format: "mac", Nullable: true}, []any{nil, "00:11:22:33:44:55"}, []any{"garbage", 42}},
		{"union formats", &Config{AnyOf: []*Config{{Type: "string", Format: "mac"}, {Type: "integer"}}}, []any{"00:11:22:33:44:55", 42}, []any{"garbage"}},
		{"outer nullable", &Config{Nullable: true, AnyOf: []*Config{{Type: "integer"}, {Type: "string", Enum: []any{"auto"}}}}, []any{nil, 42, "auto"}, []any{true, "bad"}},
		{"nullable enum", &Config{Type: "string", Nullable: true, Enum: []any{"auto"}}, []any{nil, "auto"}, []any{"bad"}},
		{"nested wrapper bounds", &Config{AnyOf: []*Config{{MaxLength: &maxLength, AnyOf: []*Config{{Type: "string"}}}, {Type: "integer"}}}, []any{"abc", 42}, []any{"abcd"}},
		{"exclusive union", &Config{OneOf: []*Config{{Type: "number"}, {Type: "integer"}}}, []any{1.5}, []any{1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := compileJSONSchema(t, tc.schema)
			for _, v := range tc.good {
				if err := schema.Validate(v); err != nil {
					t.Errorf("rejects valid %#v: %v", v, err)
				}
			}
			for _, v := range tc.bad {
				if err := schema.Validate(v); err == nil {
					t.Errorf("accepts invalid %#v", v)
				}
			}
		})
	}
}

func compileJSONSchema(t *testing.T, schema *Config) *jsonschema.Schema {
	t.Helper()
	data, err := json.Marshal(schema.JSONSchemaDoc())
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func TestOpenAPIFormattedUnionReferences(t *testing.T) {
	var buf bytes.Buffer
	schema := &Config{AnyOf: []*Config{{Type: "string", Format: "mac"}, {Type: "integer"}}, Nullable: true}
	if err := WriteOpenAPI(&buf, schema, nil); err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("openapi.json", doc); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile("openapi.json#/components/schemas/NvueConfig")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{nil, "00:11:22:33:44:55", 42} {
		if err := compiled.Validate(value); err != nil {
			t.Error(err)
		}
	}
	if err := compiled.Validate("bad"); err == nil {
		t.Error("accepted invalid MAC")
	}
}
