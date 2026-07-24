package nvueschema

import (
	"cmp"
	"slices"
	"strings"
)

// propertyEntry holds a property name and its schema, used when iterating
// over sorted properties in output generators.
type propertyEntry struct {
	name   string
	schema *Config
}

// sortedProperties returns the properties of a schema as a sorted slice.
func sortedProperties(s *Config) []propertyEntry {
	if s == nil || s.Properties == nil {
		return nil
	}
	props := make([]propertyEntry, 0, len(s.Properties))
	for k, v := range s.Properties {
		props = append(props, propertyEntry{k, v})
	}
	slices.SortFunc(props, func(a, b propertyEntry) int {
		return cmp.Compare(a.name, b.name)
	})
	return props
}

// scalarUnionVariants returns the anyOf or oneOf variants for a scalar union,
// preferring anyOf. Returns nil if neither is set.
//
// Nested scalar unions are flattened recursively. NVUE commonly expresses a
// value such as integer | "none" | "auto" as:
//
//	anyOf:
//	  - $ref: integer | "none"
//	  - $ref: "auto"
//
// Leaving the inner union nested makes isScalarUnion reject the outer union,
// after which FlattenComposite collapses it to whichever scalar type appears
// first.
func scalarUnionVariants(s *Config) []*Config {
	variants := s.AnyOf
	if len(variants) == 0 {
		variants = s.OneOf
	}

	var expanded []*Config
	for _, variant := range variants {
		if inner := scalarUnionVariants(variant); len(inner) > 0 {
			expanded = append(expanded, inner...)
			continue
		}
		expanded = append(expanded, variant)
	}
	return expanded
}

// splitIdentifier splits a string on '-', '_', and '.' delimiters.
func splitIdentifier(s string) []string {
	return strings.FieldsFunc(s, func(c rune) bool {
		return c == '-' || c == '_' || c == '.'
	})
}

// toPascal converts a kebab-case or snake_case name to PascalCase.
func toPascal(s string) string {
	parts := splitIdentifier(s)
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
