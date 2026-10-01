package nvueschema

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

// WriteJSONSchema outputs the config schema as a standalone JSON Schema draft-07 document.
func WriteJSONSchema(w io.Writer, schema *Config, info map[string]any) error {
	title := "Cumulus Linux NVUE Configuration"
	if v, ok := info["title"].(string); ok {
		title = v
	}

	doc := schema.JSONSchemaDoc()
	doc["title"] = title
	if v, ok := info["version"].(string); ok {
		doc["$comment"] = fmt.Sprintf("Generated from NVUE OpenAPI spec version %s", v)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// JSONSchemaDoc builds a complete JSON Schema 2020-12 document,
// including $schema and $defs for format types.
func (s *Config) JSONSchemaDoc() map[string]any {
	doc := s.ToJSONSchema()
	doc["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	doc["$defs"] = formatDefs()
	return doc
}

// ToJSONSchema converts a Schema to a JSON Schema map.
func (s *Config) ToJSONSchema() map[string]any {
	if s == nil {
		return map[string]any{}
	}

	// For scalar unions, emit directly without flattening.
	if isScalarUnion(s) {
		return scalarUnionToJSONSchema(s)
	}

	flat := FlattenComposite(s)
	out := map[string]any{}

	// Source path as comment.
	ref := sourceRefFor(s)
	if ref != "" {
		out["$comment"] = fmt.Sprintf("Path: %s", ref)
	}

	if flat.Description != "" {
		out["description"] = flat.Description
	}

	// Determine type, applying format overrides.
	typ := flat.Type
	if typ == "" && hasProps(flat) {
		typ = "object"
	}
	if typ == "" && flat.AdditionalProperties != nil {
		typ = "object"
	}

	// A format reference is an additional constraint, not a replacement
	// for bounds, enums, patterns, or annotations on this field.
	if jsDef := formatToJSONSchemaDef(flat.Format); jsDef != "" {
		ref := map[string]any{"$ref": "#/$defs/" + jsDef}
		if flat.Nullable {
			out["anyOf"] = []map[string]any{ref, {"type": "null"}}
		} else {
			out["$ref"] = ref["$ref"]
		}
	}

	if typ != "" {
		out["type"] = typ
	}

	if flat.Nullable {
		// JSON Schema draft-07 nullable via type array.
		if typ != "" {
			out["type"] = []string{typ, "null"}
		}
	}

	// Enum
	if len(flat.Enum) > 0 {
		values := slices.Clone(flat.Enum)
		if flat.Nullable && !slices.ContainsFunc(values, func(v any) bool { return v == nil }) {
			values = append(values, nil)
		}
		out["enum"] = values
	}

	// Numeric constraints.
	if flat.Minimum != nil {
		out["minimum"] = *flat.Minimum
	}
	if flat.Maximum != nil {
		out["maximum"] = *flat.Maximum
	}

	// String constraints.
	if flat.MinLength != nil {
		out["minLength"] = *flat.MinLength
	}
	if flat.MaxLength != nil {
		out["maxLength"] = *flat.MaxLength
	}
	if flat.Pattern != "" {
		out["pattern"] = flat.Pattern
	}
	if flat.Format != "" {
		out["format"] = flat.Format
	}

	// Default
	if flat.Default != nil {
		out["default"] = flat.Default
	}

	// Required
	if len(flat.Required) > 0 {
		out["required"] = flat.Required
	}

	// Properties
	if hasProps(flat) {
		props := map[string]any{}
		for k, v := range flat.Properties {
			props[k] = v.ToJSONSchema()
		}
		out["properties"] = props
		out["additionalProperties"] = false
	}

	// additionalProperties (dict-like)
	if flat.AdditionalProperties != nil && !hasProps(flat) {
		out["additionalProperties"] = flat.AdditionalProperties.ToJSONSchema()
	}

	// Items (array)
	if flat.Items != nil {
		out["items"] = flat.Items.ToJSONSchema()
	}

	return out
}

func scalarUnionToJSONSchema(s *Config) map[string]any {
	// Preserve constraints on wrappers as well as on the leaf branches.
	base := *s
	base.AnyOf, base.OneOf = nil, nil
	out := base.ToJSONSchema()
	key, variants := "anyOf", s.AnyOf
	if len(variants) == 0 {
		key, variants = "oneOf", s.OneOf
	}
	var schemas []map[string]any
	for _, v := range variants {
		branch := v.ToJSONSchema()
		// Flatten only a bare anyOf. Constraints and nullability attached to
		// intermediate wrappers must remain in force; oneOf is exclusive.
		if inner, ok := branch["anyOf"].([]map[string]any); key == "anyOf" && ok && len(branch) == 1 {
			schemas = append(schemas, inner...)
		} else {
			schemas = append(schemas, branch)
		}
	}
	if s.Nullable {
		if key == "oneOf" {
			schemas = []map[string]any{{"oneOf": schemas}}
			key = "anyOf"
		}
		schemas = append(schemas, map[string]any{"type": "null"})
	}
	composition := map[string]any{key: schemas}
	if len(schemas) == 1 {
		composition = schemas[0]
	}
	for k := range composition {
		if _, exists := out[k]; exists && k != "description" && k != "default" && k != "$comment" {
			out["allOf"] = []map[string]any{composition}
			return out
		}
	}
	for k, v := range composition {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
	return out
}

// formatToJSONSchemaDef returns the $defs key for a format, or "" if not mapped.
func formatToJSONSchemaDef(format string) string {
	k := formatKeyFor(format)
	if k == 0 {
		return ""
	}
	if t, ok := jsonSchemaDefTypes[k]; ok {
		return t
	}
	return ""
}

var jsonSchemaDefTypes = map[formatKey]string{
	fmtIPv4Addr:           "ipv4-address",
	fmtIPv6Addr:           "ipv6-address",
	fmtIPAddr:             "ip-address",
	fmtIPv4Prefix:         "ipv4-prefix",
	fmtIPv6Prefix:         "ipv6-prefix",
	fmtMAC:                "mac-address",
	fmtInterfaceName:      "interface-name",
	fmtVrfName:            "vrf-name",
	fmtVlanRange:          "vlan-range",
	fmtPortRange:          "port-range",
	fmtRouteDistinguisher: "route-distinguisher",
	fmtRouteTarget:        "route-target",
	fmtExtCommunity:       "ext-community",
	fmtBgpCommunity:       "bgp-community",
	fmtEvpnRoute:          "evpn-route",
	fmtAsnRange:           "asn-range",
	fmtEsIdentifier:       "es-identifier",
	fmtSegmentIdentifier:  "segment-identifier",
	fmtBgpRegex:           "bgp-regex",
	fmtHostname:           "hostname",
	fmtUserName:           "user-name",
	fmtSnmpOid:            "snmp-oid",
	fmtSecretString:       "secret-string",
	fmtKeyString:          "key-string",
}

// formatDefs returns the $defs block with pattern-validated format types.
func formatDefs() map[string]any {
	defs := map[string]any{
		"ipv4-address": map[string]any{
			"type":   "string",
			"format": "ipv4",
		},
		"ipv6-address": map[string]any{
			"type":   "string",
			"format": "ipv6",
		},
		"ipv4-prefix": map[string]any{
			"type":    "string",
			"pattern": `^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/\d{1,2}$`,
		},
		"ipv6-prefix": map[string]any{
			"type":    "string",
			"pattern": `^[0-9a-fA-F:]+/\d{1,3}$`,
		},
		"ip-address": map[string]any{
			"anyOf": []map[string]any{
				{"type": "string", "format": "ipv4"},
				{"type": "string", "format": "ipv6"},
			},
		},
		"mac-address": map[string]any{
			"type":    "string",
			"pattern": `^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`,
		},
		"interface-name": map[string]any{
			"type":    "string",
			"pattern": `^(swp|eth|bond|br|lo|vlan|peerlink|erspan|mgmt)[a-zA-Z0-9_./-]*$`,
		},
		"vrf-name": map[string]any{
			"type":    "string",
			"pattern": `^[a-zA-Z][a-zA-Z0-9_-]{0,14}$`,
		},
		"vlan-range": map[string]any{
			"type":    "string",
			"pattern": `^[0-9]+(-[0-9]+)?(,[0-9]+(-[0-9]+)?)*$`,
		},
		"port-range": map[string]any{
			"type":    "string",
			"pattern": `^[0-9]+(-[0-9]+)?$`,
		},
		"route-distinguisher": map[string]any{
			"type":    "string",
			"pattern": `^(\d+\.\d+\.\d+\.\d+:\d+|\d+:\d+)$`,
		},
		"route-target": map[string]any{
			"type":    "string",
			"pattern": `^(\d+\.\d+\.\d+\.\d+:\d+|\d+:\d+)$`,
		},
		"ext-community": map[string]any{
			"type":    "string",
			"pattern": `^(rt|soo|bandwidth)\s+\S+$`,
		},
		"bgp-community": map[string]any{
			"type":    "string",
			"pattern": `^(\d+:\d+|no-export|no-advertise|local-AS|no-peer|blackhole|graceful-shutdown|accept-own|internet)$`,
		},
		"evpn-route": map[string]any{
			"type": "string",
			"enum": []string{"macip", "imet", "prefix"},
		},
		"bgp-regex": map[string]any{
			"type":      "string",
			"minLength": 1,
		},
		"asn-range": map[string]any{
			"type":    "string",
			"pattern": `^(\d+|\d+-\d+)(,(\d+|\d+-\d+))*$`,
		},
		"es-identifier": map[string]any{
			"type":    "string",
			"pattern": `^([0-9A-Fa-f]{2}:){9}[0-9A-Fa-f]{2}$`,
		},
		"segment-identifier": map[string]any{
			"type":    "string",
			"pattern": `^\d+$`,
		},
		"hostname": map[string]any{
			"type":    "string",
			"format":  "hostname",
			"pattern": `^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`,
		},
		"user-name": map[string]any{
			"type":    "string",
			"pattern": `^[a-z_][a-z0-9_-]*[$]?$`,
		},
		"snmp-oid": map[string]any{
			"type":    "string",
			"pattern": `^\.?(\d+\.)*\d+$`,
		},
		"secret-string": map[string]any{
			"type":      "string",
			"maxLength": 64,
		},
		"key-string": map[string]any{
			"type": "string",
		},
	}
	return defs
}
