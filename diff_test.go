package nvueschema

import (
	"strings"
	"testing"
)

func TestDiffSchemaCollectionsAndConstraints(t *testing.T) {
	obj := func(value *Config) *Config { return &Config{Properties: map[string]*Config{"value": value}} }
	number := 10.0
	for _, tc := range []struct {
		name       string
		old, new   *Config
		path, desc string
	}{
		{"array items", obj(&Config{Type: "array", Items: &Config{Type: "integer"}}), obj(&Config{Type: "array", Items: &Config{Type: "string"}}), "root.value.[]", "type: integer -> string"},
		{"map values", obj(&Config{AdditionalProperties: &Config{Type: "integer"}}), obj(&Config{AdditionalProperties: &Config{Type: "string"}}), "root.value.[*]", "type: integer -> string"},
		{"item bounds", obj(&Config{Type: "array", Items: &Config{Type: "integer"}}), obj(&Config{Type: "array", Items: &Config{Type: "integer", Maximum: &number}}), "root.value.[]", "maximum:"},
		{"value enums", obj(&Config{AdditionalProperties: &Config{Type: "string", Enum: []any{"a", "b"}}}), obj(&Config{AdditionalProperties: &Config{Type: "string", Enum: []any{"a"}}}), "root.value.[*]", "enum removed:"},
		{"nullable", obj(&Config{Type: "string", Nullable: true}), obj(&Config{Type: "string"}), "root.value", "nullable: true -> false"},
		{"required", obj(&Config{Type: "string"}), &Config{Properties: map[string]*Config{"value": {Type: "string"}}, Required: []string{"value"}}, "root.value", "required: false -> true"},
		{"item object", obj(&Config{Type: "array", Items: obj(&Config{Type: "integer"})}), obj(&Config{Type: "array", Items: obj(&Config{Type: "string"})}), "root.value.[].value", "type: integer -> string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diff := DiffSchemas(tc.old, tc.new, "root")
			if len(diff.Changes) != 1 || diff.Changes[0].Path != tc.path || !strings.Contains(diff.Changes[0].Desc, tc.desc) {
				t.Fatalf("unexpected changes: %+v", diff.Changes)
			}
			if len(diff.FilterAffected([]string{"root.value"}).Changes) != 1 {
				t.Error("collection change missing from affected paths")
			}
			if got := DiffSchemas(tc.new, tc.new, "root"); len(got.Changes) != 0 {
				t.Errorf("self diff: %+v", got.Changes)
			}
		})
	}
}

func TestDiffRequiredOrderDoesNotMatter(t *testing.T) {
	old := &Config{Required: []string{"a", "b"}}
	newer := &Config{Required: []string{"b", "a", "a"}}
	if got := DiffSchemas(old, newer, ""); len(got.Changes) != 0 {
		t.Errorf("required is a set: %+v", got.Changes)
	}
}

func TestDiffCollectionSchemaAddedAndRemoved(t *testing.T) {
	for _, array := range []bool{false, true} {
		old := &Config{Type: "object"}
		newer := &Config{Type: "object", AdditionalProperties: &Config{Type: "string"}}
		wantPath := "root.[*]"
		if array {
			old = &Config{Type: "array"}
			newer = &Config{Type: "array", Items: &Config{Type: "string"}}
			wantPath = "root.[]"
		}
		for _, reverse := range []bool{false, true} {
			a, b, kind := old, newer, "added"
			if reverse {
				a, b, kind = newer, old, "removed"
			}
			found := false
			for _, change := range DiffSchemas(a, b, "root").Changes {
				if change.Path == wantPath && change.Kind == kind {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s at %s", kind, wantPath)
			}
		}
	}
}
