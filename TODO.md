# Code review follow-up

Each item gets a separate Jujutsu change, with regression tests committed alongside the fix.

- [ ] 1. Make CLI validation handle NVUE regular expressions, including negative lookahead, without weakening validation.
- [ ] 2. Fix Protobuf map-entry name collisions and message deduplication across nested scopes; compile generated fixtures.
- [ ] 3. Emit valid YANG type statements; validate generated fixtures with a YANG parser.
- [ ] 4. Preserve JSON Schema format constraints, local constraints, and nullability, including scalar unions.
- [ ] 5. Preserve Pydantic field bounds, lengths, and patterns; exercise generated models with valid and invalid values.
- [ ] 6. Detect changes to array items, map values, required properties, and nullability in schema diffs.
- [ ] 7. Preserve cache validators on the first download and revalidate cached schemas on subsequent fetches.
- [ ] 8. Visit each subtree once when expanding the browser tree; add deep-tree coverage and benchmarks.
- [ ] 9. Emit object models referenced by array items in Go, Python, and Protobuf; compile/initialize generated fixtures.
- [ ] 10. Propagate writer errors from Pydantic, YANG, and Protobuf generation, including short writes.
