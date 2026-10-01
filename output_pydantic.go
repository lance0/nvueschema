package nvueschema

import (
	"fmt"
	"io"
	"math"
	"strings"
)

// WritePydantic outputs the config schema as Python Pydantic v2 model classes.
func WritePydantic(w io.Writer, schema *Config, info map[string]any) error {
	g := &pyGen{
		w:      w,
		models: make(map[string]bool),
	}

	fmt.Fprintln(w, `"""`)
	fmt.Fprintln(w, `Cumulus Linux NVUE Configuration Schema — Pydantic v2 Models`)
	if v, ok := info["version"].(string); ok {
		fmt.Fprintf(w, "Generated from NVUE OpenAPI spec version %s\n", v)
	}
	fmt.Fprintln(w, `"""`)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "from __future__ import annotations")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "import re")
	fmt.Fprintln(w, "from functools import partial")
	fmt.Fprintln(w, "from datetime import date, datetime, time")
	fmt.Fprintln(w, "from ipaddress import IPv4Address, IPv4Network, IPv6Address, IPv6Network")
	fmt.Fprintln(w, "from typing import Annotated, Any, Dict, List, Literal, Optional, Union")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "from pydantic import AnyUrl, BaseModel, BeforeValidator, Field, FilePath, SecretStr")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "# Validated network configuration types")
	for _, td := range typedefs {
		if td.pattern != "" {
			fmt.Fprintf(w, "%s = Annotated[str, Field(pattern=r\"%s\")]\n", td.name, td.pattern)
		} else {
			fmt.Fprintf(w, "%s = Annotated[str, Field(min_length=1)]\n", td.name)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprint(w, `def _validate_string(value: Any, *, min_length: int = 0, max_length: Optional[int] = None, pattern: Optional[str] = None) -> Any:
    if isinstance(value, str):
        if len(value) < min_length or (max_length is not None and len(value) > max_length):
            raise ValueError("string length outside schema bounds")
        if pattern is not None and re.search(pattern, value) is None:
            raise ValueError("string does not match schema pattern")
    return value
`)
	g.emitModel("NvueConfig", schema)

	return nil
}

type pyGen struct {
	w      io.Writer
	models map[string]bool
}

func (g *pyGen) emitModel(name string, s *Config) {
	if g.models[name] {
		return
	}
	g.models[name] = true

	merged := FlattenComposite(s)
	props := sortedProperties(merged)

	// Discover dependencies through arrays and maps before emitting fields.
	for _, p := range props {
		g.emitTypeModels(name+toPascal(p.name), p.schema)
	}

	requiredSet := make(map[string]bool)
	for _, r := range merged.Required {
		requiredSet[r] = true
	}

	fmt.Fprintf(g.w, "\nclass %s(BaseModel):\n", name)
	docParts := []string{}
	if merged.Description != "" {
		docParts = append(docParts, sanitizeDocstring(merged.Description))
	}
	sourceRef := sourceRefFor(s)
	if sourceRef != "" {
		docParts = append(docParts, fmt.Sprintf("Path: %s", sourceRef))
	}
	if len(docParts) > 0 {
		fmt.Fprintln(g.w, "    \"\"\"")
		for _, part := range docParts {
			fmt.Fprintf(g.w, "    %s\n", part)
		}
		fmt.Fprintln(g.w, "    \"\"\"")
	}

	if len(props) == 0 {
		fmt.Fprintln(g.w, "    pass")
		fmt.Fprintln(g.w)
		return
	}

	for _, p := range props {
		flat := FlattenComposite(p.schema)
		pyType := g.pyType(name+toPascal(p.name), p.schema)
		fieldName := toSnake(p.name)
		alias := ""
		if fieldName != p.name {
			alias = fmt.Sprintf(", alias=%q", p.name)
		}

		if requiredSet[p.name] {
			if flat.Description != "" {
				fmt.Fprintf(g.w, "    %s: %s = Field(...%s, description=%q)\n", fieldName, pyType, alias, flat.Description)
			} else {
				fmt.Fprintf(g.w, "    %s: %s = Field(...%s)\n", fieldName, pyType, alias)
			}
		} else {
			if flat.Description != "" {
				fmt.Fprintf(g.w, "    %s: Optional[%s] = Field(None%s, description=%q)\n", fieldName, pyType, alias, flat.Description)
			} else {
				fmt.Fprintf(g.w, "    %s: Optional[%s] = Field(None%s)\n", fieldName, pyType, alias)
			}
		}
	}
	fmt.Fprintln(g.w)
}

func (g *pyGen) emitTypeModels(name string, s *Config) {
	if s == nil || isScalarUnion(s) {
		return
	}
	flat := FlattenComposite(s)
	if flat.Type == "array" {
		g.emitTypeModels(name+"Item", flat.Items)
	} else if hasProps(flat) {
		g.emitModel(name, s)
	} else if flat.AdditionalProperties != nil {
		g.emitTypeModels(name+"Entry", flat.AdditionalProperties)
	}
}

func (g *pyGen) pyType(contextName string, s *Config) string {
	if s == nil {
		return "Any"
	}

	// Handle anyOf/oneOf with only scalar branches as Union.
	if isScalarUnion(s) {
		return constrainedPyType(scalarUnionType(s), s)
	}

	flat := FlattenComposite(s)

	if len(flat.Enum) > 0 {
		return scalarPyType(flat)
	}

	switch flat.Type {
	case "string", "integer", "number", "boolean":
		return scalarPyType(flat)
	case "array":
		inner := g.pyType(contextName+"Item", flat.Items)
		return fmt.Sprintf("List[%s]", inner)
	case "object":
		if hasProps(flat) {
			return contextName
		}
		if flat.AdditionalProperties != nil {
			apFlat := FlattenComposite(flat.AdditionalProperties)
			if hasProps(apFlat) {
				valType := g.pyType(contextName+"Entry", flat.AdditionalProperties)
				return fmt.Sprintf("Dict[str, %s]", valType)
			}
			return "Dict[str, Any]"
		}
		return "Dict[str, Any]"
	}

	if hasProps(flat) {
		return contextName
	}
	if flat.AdditionalProperties != nil {
		apFlat := FlattenComposite(flat.AdditionalProperties)
		if hasProps(apFlat) {
			valType := g.pyType(contextName+"Entry", flat.AdditionalProperties)
			return fmt.Sprintf("Dict[str, %s]", valType)
		}
		return "Dict[str, Any]"
	}

	return "Any"
}

// scalarUnionType builds a Union[...] type string from scalar anyOf/oneOf branches.
func scalarUnionType(s *Config) string {
	variants := s.AnyOf
	if len(variants) == 0 {
		variants = s.OneOf
	}
	seen := make(map[string]bool)
	var types []string
	for _, v := range variants {
		t := scalarPyType(v)
		if !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	if len(types) == 1 {
		return types[0]
	}
	return fmt.Sprintf("Union[%s]", strings.Join(types, ", "))
}

func scalarPyType(s *Config) string {
	if isScalarUnion(s) {
		return constrainedPyType(scalarUnionType(s), s)
	}
	return constrainedPyType(scalarPyBaseType(s), s)
}

func constrainedPyType(typ string, s *Config) string {
	var metadata, bounds, stringsBounds []string
	if s.Minimum != nil {
		if typ == "int" {
			bounds = append(bounds, fmt.Sprintf("ge=%.0f", math.Ceil(*s.Minimum)))
		} else {
			bounds = append(bounds, fmt.Sprintf("ge=%g", *s.Minimum))
		}
	}
	if s.Maximum != nil {
		if typ == "int" {
			bounds = append(bounds, fmt.Sprintf("le=%.0f", math.Floor(*s.Maximum)))
		} else {
			bounds = append(bounds, fmt.Sprintf("le=%g", *s.Maximum))
		}
	}
	if len(bounds) > 0 {
		metadata = append(metadata, "Field("+strings.Join(bounds, ", ")+")")
	}
	if s.MinLength != nil {
		stringsBounds = append(stringsBounds, fmt.Sprintf("min_length=%d", *s.MinLength))
	}
	if s.MaxLength != nil {
		stringsBounds = append(stringsBounds, fmt.Sprintf("max_length=%d", *s.MaxLength))
	}
	if s.Pattern != "" {
		stringsBounds = append(stringsBounds, fmt.Sprintf("pattern=%q", s.Pattern))
	}
	// Validate the original string before parsing format types such as IP
	// addresses. This also keeps local patterns from replacing a typedef's
	// own pattern, and Python's re supports NVUE lookahead expressions.
	if len(stringsBounds) > 0 {
		metadata = append(metadata, "BeforeValidator(partial(_validate_string, "+strings.Join(stringsBounds, ", ")+"))")
	}
	if len(metadata) > 0 {
		typ = "Annotated[" + typ + ", " + strings.Join(metadata, ", ") + "]"
	}
	if s.Nullable {
		typ = "Optional[" + typ + "]"
	}
	return typ
}

func scalarPyBaseType(s *Config) string {
	if len(s.Enum) > 0 {
		var vals []string
		for _, e := range s.Enum {
			// skip null/nil enum values — handled by Optional wrapper
			if v, ok := e.(string); ok {
				vals = append(vals, fmt.Sprintf("%q", v))
			}
		}
		if len(vals) == 0 {
			return "None"
		}
		if len(vals) == 1 {
			return fmt.Sprintf("Literal[%s]", vals[0])
		}
		return fmt.Sprintf("Literal[%s]", strings.Join(vals, ", "))
	}
	if t := formatToPyType(s.Format); t != "" {
		return t
	}
	switch s.Type {
	case "string":
		return "str"
	case "integer":
		return "int"
	case "number":
		return "float"
	case "boolean":
		return "bool"
	default:
		return "Any"
	}
}

// formatToPyType maps OpenAPI format strings to Python/Pydantic types via the format registry.
func formatToPyType(format string) string {
	k := formatKeyFor(format)
	if k == 0 {
		return ""
	}
	if t, ok := pyFormatTypes[k]; ok {
		return t
	}
	return ""
}

var pyFormatTypes = map[formatKey]string{
	fmtIPv4Addr:           "IPv4Address",
	fmtIPv6Addr:           "IPv6Address",
	fmtIPAddr:             "Union[IPv4Address, IPv6Address]",
	fmtIPv4Prefix:         "IPv4Network",
	fmtIPv6Prefix:         "IPv6Network",
	fmtMAC:                "MacAddress",
	fmtInterfaceName:      "InterfaceName",
	fmtVrfName:            "VrfName",
	fmtVlanRange:          "VlanRange",
	fmtPortRange:          "PortRange",
	fmtRouteDistinguisher: "RouteDistinguisher",
	fmtRouteTarget:        "RouteTarget",
	fmtExtCommunity:       "ExtCommunity",
	fmtBgpCommunity:       "BgpCommunity",
	fmtEvpnRoute:          "EvpnRoute",
	fmtBgpRegex:           "BgpRegex",
	fmtAsnRange:           "AsnRange",
	fmtEsIdentifier:       "EsIdentifier",
	fmtSegmentIdentifier:  "SegmentIdentifier",
	fmtHostname:           "Hostname",
	fmtUserName:           "UserName",
	fmtSnmpOid:            "SnmpOid",
	fmtSecretString:       "SecretStr",
	fmtInteger:            "int",
	fmtFloat:              "float",
	fmtDateTime:           "datetime",
	fmtClockDate:          "date",
	fmtClockTime:          "time",
	fmtGenericName:        "str",
	fmtFileName:           "FilePath",
	fmtRepoURL:            "AnyUrl",
	fmtRepoDist:           "str",
	fmtJSONPointer:        "str",
	fmtClockID:            "str",
	fmtSequenceID:         "int",
	fmtCommand:            "str",
	fmtInterval:           "str",
}

var pyReserved = map[string]bool{
	"False": true, "None": true, "True": true, "and": true, "as": true,
	"assert": true, "async": true, "await": true, "break": true, "class": true,
	"continue": true, "def": true, "del": true, "elif": true, "else": true,
	"except": true, "finally": true, "for": true, "from": true, "global": true,
	"if": true, "import": true, "in": true, "is": true, "lambda": true,
	"nonlocal": true, "not": true, "or": true, "pass": true, "raise": true,
	"return": true, "try": true, "while": true, "with": true, "yield": true,
	"type": true,
}

func toSnake(s string) string {
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	if pyReserved[s] {
		s += "_"
	}
	return s
}

func sanitizeDocstring(s string) string {
	return strings.ReplaceAll(s, `"""`, `\"\"\"`)
}
