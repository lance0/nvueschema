package yangregexp

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

// Exercise the real YANG/XSD engine, independently of the fast Go oracle used
// by exhaustive and fuzz tests. An explicit interpreter makes this mandatory.
func TestYANGEquivalence(t *testing.T) {
	python := os.Getenv("NVUESCHEMA_PYTHON")
	explicit := python != ""
	if !explicit {
		python = "python3"
	}
	if out, err := exec.Command(python, "-c", "import pyang, lxml.etree").CombinedOutput(); err != nil {
		if explicit {
			t.Fatalf("Python validators: %v\n%s", err, out)
		}
		t.Skip("pyang/lxml unavailable; set NVUESCHEMA_PYTHON to require this test")
	}
	type fixture struct {
		Source   string
		Patterns []Pattern
		Values   []string
		Matches  []bool
	}
	var fixtures []fixture
	for _, source := range allPatterns(t) {
		ps, err := Convert(source)
		if err != nil {
			if source == `^(?=a(?=b)).*$` {
				continue
			}
			t.Fatal(err)
		}
		row := fixture{Source: source, Patterns: ps, Values: candidates()}
		original := sourceMatcher(t, source)
		for _, value := range row.Values {
			match, err := original.MatchString(value)
			if err != nil {
				t.Fatal(err)
			}
			row.Matches = append(row.Matches, match)
		}
		fixtures = append(fixtures, row)
	}
	data, err := json.Marshal(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "-c", `
import json, sys
from xml.etree import ElementTree as ET
from pyang import context, repository, error
rows = json.load(sys.stdin)
ns = 'urn:ietf:params:xml:ns:yang:yin:1'
def add(parent, tag, **attrs):
    return ET.SubElement(parent, '{%s}%s' % (ns, tag), attrs)
source = ET.Element('{%s}module' % ns, name='check')
add(source, 'yang-version', value='1.1')
add(source, 'namespace', uri='urn:check')
add(source, 'prefix', value='c')
for i, row in enumerate(rows):
    typ = add(add(source, 'leaf', name='f%d' % i), 'type', name='string')
    for pattern in row['Patterns']:
        p = add(typ, 'pattern', value=pattern['Expression'])
        if pattern['InvertMatch']:
            add(p, 'modifier', value='invert-match')
ctx = context.Context(repository.FileRepository('.'))
module = ctx.add_module('check.yin', ET.tostring(source, encoding='unicode'), in_format='yin')
ctx.validate()
failures = [entry for entry in ctx.errors if error.is_error(error.err_level(entry[1]))]
assert not failures, failures
for i, row in enumerate(rows):
    spec = module.search_one('leaf', 'f%d' % i).search_one('type').i_type_spec
    for value, expected in zip(row['Values'], row['Matches']):
        errors = []
        actual = spec.validate(errors, module.pos, value, module)
        assert actual == expected, (row['Source'], value, expected, actual, row['Patterns'], errors)
print('%d source patterns verified through pyang' % len(rows))
`)
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("YANG comparison: %v\n%s", err, out)
	}
	t.Log(string(out))
}
