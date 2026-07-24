package nvueschema

import (
	"encoding/json"
	"strings"
	"testing"
)

// A scalar union nested inside a single-branch anyOf wrapper (the OpenAPI
// idiom `{"anyOf":[{"$ref":...}],"nullable":true}` used to attach nullable to
// a $ref) must not be collapsed to its first branch. This mirrors the NVUE
// acl match/ip/protocol def, which is integer(0-255) | string enum
// (tcp,udp,...); collapsing it to integer alone wrongly rejects `protocol: tcp`.
func TestScalarUnion_NestedInSingleBranchWrapper_Preserved(t *testing.T) {
	min0, max255 := 0.0, 255.0
	protocol := &Config{
		Description: "IP protocol",
		Nullable:    true,
		AnyOf: []*Config{ // single-branch wrapper around the $ref target
			{
				AnyOf: []*Config{ // the referenced scalar union
					{Type: "integer", Minimum: &min0, Maximum: &max255, Nullable: true},
					{Type: "string", Enum: []any{"tcp", "udp", "icmp"}, Nullable: true},
				},
			},
		},
	}

	out := protocol.ToJSONSchema()

	// Must not have collapsed to a lone integer scalar.
	if _, collapsed := out["minimum"]; collapsed {
		t.Fatalf("union collapsed to a top-level integer scalar: %#v", out)
	}
	variants, ok := out["anyOf"].([]map[string]any)
	if !ok || len(variants) != 2 {
		t.Fatalf("expected anyOf with 2 variants, got %#v", out["anyOf"])
	}

	blob, _ := json.Marshal(out)
	s := string(blob)
	if !strings.Contains(s, "integer") {
		t.Error("integer branch lost")
	}
	if !strings.Contains(s, "\"tcp\"") {
		t.Error("string-enum branch (tcp) lost — this is the bug")
	}
	// Integer branch keeps its numeric bounds.
	if !strings.Contains(s, "255") {
		t.Error("integer branch dropped its maximum constraint")
	}
}

// NVUE models peer-group BGP timers as an outer union whose first branch is
// itself a union: (integer | "none") | "auto". The integer branch must survive
// conversion to JSON Schema.
func TestScalarUnion_NestedMultiBranchTimer_Preserved(t *testing.T) {
	min3, max65535 := 3.0, 65535.0
	timer := &Config{
		Description: "Hold timer",
		Default:     "auto",
		AnyOf: []*Config{
			{
				AnyOf: []*Config{
					{Type: "string", Enum: []any{"none", nil}, Nullable: true},
					{Type: "integer", Minimum: &min3, Maximum: &max65535, Nullable: true},
				},
			},
			{Type: "string", Enum: []any{"auto", nil}, Nullable: true},
		},
	}

	out := timer.ToJSONSchema()
	variants, ok := out["anyOf"].([]map[string]any)
	if !ok || len(variants) != 3 {
		t.Fatalf("expected 3 flattened scalar variants, got %#v", out)
	}

	blob, _ := json.Marshal(out)
	s := string(blob)
	if !strings.Contains(s, `"type":["integer","null"]`) {
		t.Errorf("integer timer branch lost: %s", s)
	}
	if !strings.Contains(s, `"minimum":3`) || !strings.Contains(s, `"maximum":65535`) {
		t.Errorf("integer timer bounds lost: %s", s)
	}
	if !strings.Contains(s, `"none"`) || !strings.Contains(s, `"auto"`) {
		t.Errorf("timer string alternatives lost: %s", s)
	}
}
