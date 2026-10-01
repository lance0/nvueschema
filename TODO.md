# Code review follow-up

Each item gets a separate Jujutsu change, with regression tests committed alongside the fix.

- [x] 1. Make CLI validation handle NVUE regular expressions, including negative lookahead, without weakening validation.
- [x] 2. Fix Protobuf map-entry name collisions and message deduplication across nested scopes; compile generated fixtures.
- [x] 3. Emit valid YANG type statements; validate generated fixtures with a YANG parser.
- [x] 4. Preserve JSON Schema format constraints, local constraints, and nullability, including scalar unions.
- [x] 5. Preserve Pydantic field bounds, lengths, and patterns (including numeric bounds on mixed unions); exercise generated models with valid and invalid values.
- [x] 6. Detect changes to array items, map values, required properties, and nullability in schema diffs.
- [x] 7. Preserve cache validators on the first download and revalidate cached schemas on subsequent fetches.
- [x] 8. Visit each subtree once when expanding the browser tree; add deep-tree coverage and benchmarks.
- [x] 9. Emit object models referenced by array items in Go, Python, and Protobuf; compile/initialize generated fixtures.
- [x] 10. Propagate writer errors from Pydantic, YANG, and Protobuf generation, including short writes.

## Follow-up discovered during full-schema verification

- [x] Escape source patterns when emitting YANG string literals. Regression
  tests parse generated YANG with `pyang` and compare the recovered pattern
  values, covering both quote types, backslashes, newlines, tabs, and Unicode.
  The complete 5.16 schema now passes statement parsing, exposing the separate
  validation failures below.

Full-schema validation with pyang 2.7.1 still reports 566 errors (many instances
of the same underlying defects) and one unused-import warning:

- [ ] Fix YANG default serialization. String defaults are quoted twice, causing
  enum and pattern validation failures. Test parsed defaults and their validity.
- [ ] Emit valid YANG numeric ranges. Generated ranges contain Go constants such
  as `INT32_MAX`, exceed int64 bounds, or have reversed endpoints. Cover boundary
  values and verify how conflicting source constraints should be represented.
- [ ] Preserve non-string enum values when generating YANG enumerations; numeric
  enums currently produce empty enumeration types. Validate representative
  source fixtures with `pyang`.
- [ ] Supply decimal64 fraction-digits and compatible range restrictions, with
  parser-backed tests for fractional bounds and defaults.
- [ ] Handle source regexes that are incompatible with YANG's XML Schema regex
  dialect. Correct quoting preserves the source pattern but does not translate
  lookahead or unsupported escapes. Define supported conversions and explicit
  errors for unsupported patterns, with semantic regression tests.
- [ ] Require generator integration dependencies in CI and add representative
  NVUE source fixtures that go through parsing, generation, and validation.
  Python integrations currently skip when dependencies are absent, Protobuf
  compilation skips without protoc, and full-schema checks remain manual.
