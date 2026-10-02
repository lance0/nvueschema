package yangregexp

import (
	"strings"

	"github.com/dlclark/regexp2/v2/syntax"
)

type interval = syntax.SingleRange

// XML 1.0 characters, also the domain of YANG's XML Schema patterns.
var xmlChars = []interval{{First: 9, Last: 10}, {First: 13, Last: 13}, {First: 0x20, Last: 0xd7ff}, {First: 0xe000, Last: 0xfffd}, {First: 0x10000, Last: 0x10ffff}}

func xmlChar(ch rune) bool {
	for _, r := range xmlChars {
		if ch >= r.First && ch <= r.Last {
			return true
		}
	}
	return false
}

func literal(ch rune) string {
	if !xmlChar(ch) {
		return noRune
	}
	switch ch {
	case '^':
		return `[\^]`
	case '$':
		return `[$]`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\t':
		return `\t`
	}
	if strings.ContainsRune(`\.?*+(){}[]|`, ch) {
		return `\` + string(ch)
	}
	return string(ch)
}

func classChar(ch rune) string {
	switch ch {
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\t':
		return `\t`
	}
	if strings.ContainsRune(`\[]^-`, ch) {
		return `\` + string(ch)
	}
	return string(ch)
}

func character(n *syntax.RegexNode) string {
	if n.IsOneFamily() {
		return literal(n.Ch)
	}
	ranges := nodeRanges(n)
	if len(ranges) == 0 {
		return noRune
	}
	excluded := complement(ranges)
	if len(excluded) == 0 {
		return anyRune
	}
	encode := func(rs []interval) string {
		var b strings.Builder
		for _, r := range rs {
			// Escaped punctuation is not a portable XSD range endpoint (for
			// example [\--9]). Emit it separately from the remaining range.
			for r.First <= r.Last && classChar(r.First) != string(r.First) {
				b.WriteString(classChar(r.First))
				r.First++
			}
			suffix := ""
			for r.Last >= r.First && classChar(r.Last) != string(r.Last) {
				suffix = classChar(r.Last) + suffix
				r.Last--
			}
			if r.First <= r.Last {
				b.WriteString(classChar(r.First))
			}
			if r.Last > r.First+1 {
				b.WriteByte('-')
			}
			if r.Last > r.First {
				b.WriteString(classChar(r.Last))
			}
			b.WriteString(suffix)
		}
		return b.String()
	}
	positive, negative := encode(ranges), encode(excluded)
	if len(negative)+1 < len(positive) {
		return "[^" + negative + "]"
	}
	return "[" + positive + "]"
}

func nodeRanges(n *syntax.RegexNode) []interval {
	if n.IsOneFamily() || n.IsNotoneFamily() {
		rs := intersect([]interval{{First: n.Ch, Last: n.Ch}}, xmlChars)
		if n.IsNotoneFamily() {
			return complement(rs)
		}
		return rs
	}
	set := n.Set
	if set.IsAnything() {
		return xmlChars
	}
	if set.IsEmpty() {
		if set.IsNegated() {
			return xmlChars
		}
		return nil
	}
	// The parser exposes range access but not its range count. Most classes
	// have only a handful of ranges; complex/category sets use membership below.
	for count := 1; count <= 64; count++ {
		if rs := set.GetIfNRanges(count); rs != nil {
			rs = intersect(rs, xmlChars)
			if set.IsNegated() {
				return complement(rs)
			}
			return rs
		}
	}
	var result []interval
	for _, domain := range xmlChars {
		for ch := domain.First; ch <= domain.Last; ch++ {
			if !set.CharIn(ch) {
				continue
			}
			first := ch
			for ch < domain.Last && set.CharIn(ch+1) {
				ch++
			}
			result = append(result, interval{First: first, Last: ch})
		}
	}
	return result
}

func intersect(a, b []interval) []interval {
	var result []interval
	for i, j := 0, 0; i < len(a) && j < len(b); {
		lo, hi := max(a[i].First, b[j].First), min(a[i].Last, b[j].Last)
		if lo <= hi {
			result = append(result, interval{First: lo, Last: hi})
		}
		if a[i].Last < b[j].Last {
			i++
		} else {
			j++
		}
	}
	return result
}

func complement(rs []interval) []interval {
	var result []interval
	for _, domain := range xmlChars {
		next := domain.First
		for _, r := range rs {
			if r.Last < next {
				continue
			}
			if r.First > domain.Last {
				break
			}
			if r.First > next {
				result = append(result, interval{First: next, Last: r.First - 1})
			}
			next = max(next, r.Last+1)
		}
		if next <= domain.Last {
			result = append(result, interval{First: next, Last: domain.Last})
		}
	}
	return result
}
