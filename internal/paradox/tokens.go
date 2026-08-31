package paradox

import (
	"regexp"
	"sort"
	"strings"
)

// TokenKind classifies the markup constructs embedded in Paradox localization
// values. Every one of them is load-bearing: a broken token means a tooltip
// that never renders, or a hard crash during script validation.
type TokenKind string

const (
	KindVariable  TokenKind = "variable"   // $VALUE$
	KindScope     TokenKind = "scope"      // [Scope.GetName]
	KindFormat    TokenKind = "format"     // #bold
	KindFormatOff TokenKind = "format_end" // #!
	KindIcon      TokenKind = "icon"       // £gold£
	KindTextIcon  TokenKind = "texticon"   // @gold!
	KindNewline   TokenKind = "newline"    // \n
)

// tokenRE matches every markup construct in one pass so that overlapping
// candidates (a $VAR$ inside a [Scope]) are never double counted. Order in the
// alternation matters: RE2 is leftmost-first across alternatives at the same
// position.
var tokenRE = regexp.MustCompile(
	`\\n` +
		`|\$[^$\n]*\$` +
		`|\[[^\]\n]*\]` +
		`|£[^£\n]*£` +
		`|@[A-Za-z0-9_]+!` +
		`|#!` +
		`|#[a-zA-Z][a-zA-Z0-9_;]*`,
)

// Token is one markup construct found in a value.
type Token struct {
	Kind TokenKind
	Text string
	// Start and End are byte offsets into the value.
	Start, End int
}

func classify(s string) TokenKind {
	switch {
	case s == `\n`:
		return KindNewline
	case s == "#!":
		return KindFormatOff
	case strings.HasPrefix(s, "$"):
		return KindVariable
	case strings.HasPrefix(s, "["):
		return KindScope
	case strings.HasPrefix(s, "£"):
		return KindIcon
	case strings.HasPrefix(s, "@"):
		return KindTextIcon
	default:
		return KindFormat
	}
}

// Tokens returns every markup token in value, in order of appearance.
func Tokens(value string) []Token {
	locs := tokenRE.FindAllStringIndex(value, -1)
	out := make([]Token, 0, len(locs))
	for _, loc := range locs {
		text := value[loc[0]:loc[1]]
		out = append(out, Token{
			Kind:  classify(text),
			Text:  text,
			Start: loc[0],
			End:   loc[1],
		})
	}
	return out
}

// TokenMultiset counts each distinct token text. The core invariant of the
// whole project is that this multiset is identical between an English source
// string and its Korean translation.
func TokenMultiset(value string) map[string]int {
	m := map[string]int{}
	for _, t := range Tokens(value) {
		m[t.Text]++
	}
	return m
}

// TokenSignature renders the multiset as a stable, comparable string. Two
// values with equal signatures have identical token multisets.
func TokenSignature(value string) string {
	m := TokenMultiset(value)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\x1f')
		}
		b.WriteString(k)
		b.WriteByte('\x1e')
		b.WriteString(itoa(m[k]))
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// TokenDiff reports which tokens the translation gained or lost relative to
// the source. Both maps are empty when the invariant holds.
func TokenDiff(source, translation string) (missing, added map[string]int) {
	src := TokenMultiset(source)
	dst := TokenMultiset(translation)
	missing, added = map[string]int{}, map[string]int{}
	for tok, n := range src {
		if d := n - dst[tok]; d > 0 {
			missing[tok] = d
		}
	}
	for tok, n := range dst {
		if d := n - src[tok]; d > 0 {
			added[tok] = d
		}
	}
	return missing, added
}
