// Package yangregexp converts regexp2 ECMAScript search expressions into YANG
// 1.1 pattern constraints (the XML Schema regular-expression dialect).
//
// Convert parses with regexp2's AST. It preserves matching over XML 1.0 strings,
// including unanchored searches, character classes, alternation, repetition, and
// positive/negative lookahead at the beginning of an anchored expression. All
// returned constraints must be applied together; InvertMatch requires YANG 1.1.
//
// This is a dialect converter, not a regex matching engine. Constructs that
// cannot be translated by this package return an error, never an approximation.
// Backreferences, lookbehind, word boundaries, interior anchors, and lookahead
// at arbitrary positions are currently unsupported. See README.md for the
// supported grammar, resource limits, and semantic test strategy.
package yangregexp
