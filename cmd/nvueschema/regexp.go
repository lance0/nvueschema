package main

import (
	"time"

	"github.com/dlclark/regexp2/v2"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// JSON Schema uses ECMAScript patterns, including lookaround and
// backreferences, which Go's standard regexp engine cannot compile.
func compilePattern(pattern string) (jsonschema.Regexp, error) {
	re, err := regexp2.Compile(pattern, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	// Bound backtracking when a schema or configuration is supplied locally.
	re.MatchTimeout = time.Second
	return schemaRegexp{re}, nil
}

type schemaRegexp struct{ re *regexp2.Regexp }

func (r schemaRegexp) String() string { return r.re.String() }
func (r schemaRegexp) MatchString(value string) bool {
	matched, err := r.re.MatchString(value)
	return err == nil && matched
}
