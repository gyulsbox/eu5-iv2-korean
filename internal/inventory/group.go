package inventory

import (
	"sort"
	"strings"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/paradox"
)

// Placeholder syntax. Before a string is grouped or handed to a translator,
// every markup token becomes ⟦T1⟧, ⟦T2⟧ … and every remaining run of digits
// becomes ⟦N1⟧, ⟦N2⟧ …. This does three jobs at once:
//
//   - It collapses the 33k WE_PERFORM_* keys, which differ only in the $key$
//     references they chain together, into a handful of templates.
//   - It keeps fragile markup out of the translator's reach entirely, so a
//     token cannot come back mangled.
//   - Placeholders are indexed rather than positional, so a translation is
//     free to reorder them, which Korean word order requires.
const (
	TokenOpen  = "⟦T"
	NumOpen    = "⟦N"
	PlaceClose = "⟧"
)

// Member is one concrete string belonging to a group.
type Member struct {
	Key string `json:"key"`
	// Tokens holds the markup tokens lifted out of this member's value, in
	// source order.
	Tokens []string `json:"tokens,omitempty"`
	// Numbers holds the digit runs lifted out of the plain-text spans, in
	// source order. Substituting both lists back into the group template
	// reproduces the value byte for byte.
	Numbers []string `json:"numbers,omitempty"`
}

// Group is a set of source strings sharing one template, and therefore needing
// only one translation between them.
type Group struct {
	Template string `json:"template"`
	Class    Class  `json:"class"`
	// Rep is the lexically first key in the group; it names the unit.
	Rep     string   `json:"rep"`
	Members []Member `json:"members"`
}

// Templatize lifts markup tokens and numbers out of a value, returning the
// template and the two substitution lists.
func Templatize(value string) (template string, tokens, numbers []string) {
	var b strings.Builder
	toks := paradox.Tokens(value)
	prev := 0
	for _, t := range toks {
		writeDigitsTemplated(&b, value[prev:t.Start], &numbers)
		tokens = append(tokens, t.Text)
		b.WriteString(TokenOpen)
		b.WriteString(itoa(len(tokens)))
		b.WriteString(PlaceClose)
		prev = t.End
	}
	writeDigitsTemplated(&b, value[prev:], &numbers)
	return b.String(), tokens, numbers
}

// writeDigitsTemplated copies a plain-text span, replacing digit runs with
// indexed number placeholders.
func writeDigitsTemplated(b *strings.Builder, span string, numbers *[]string) {
	for i := 0; i < len(span); {
		if span[i] >= '0' && span[i] <= '9' {
			j := i
			for j < len(span) && span[j] >= '0' && span[j] <= '9' {
				j++
			}
			*numbers = append(*numbers, span[i:j])
			b.WriteString(NumOpen)
			b.WriteString(itoa(len(*numbers)))
			b.WriteString(PlaceClose)
			i = j
			continue
		}
		b.WriteByte(span[i])
		i++
	}
}

// Detemplatize substitutes tokens and numbers back into a template. It reports
// false if the template references an index outside either list, which means
// the translation dropped or invented a placeholder.
func Detemplatize(template string, tokens, numbers []string) (string, bool) {
	var b strings.Builder
	ok := true
	for i := 0; i < len(template); {
		if idx, width, kind, matched := readPlaceholder(template[i:]); matched {
			list := tokens
			if kind == NumOpen {
				list = numbers
			}
			if idx >= 1 && idx <= len(list) {
				b.WriteString(list[idx-1])
			} else {
				ok = false
			}
			i += width
			continue
		}
		b.WriteByte(template[i])
		i++
	}
	return b.String(), ok
}

// readPlaceholder matches a placeholder at the start of s, returning its
// 1-based index, its byte width, and which kind it is.
func readPlaceholder(s string) (idx, width int, kind string, matched bool) {
	for _, open := range []string{TokenOpen, NumOpen} {
		if !strings.HasPrefix(s, open) {
			continue
		}
		rest := s[len(open):]
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		if j == 0 || !strings.HasPrefix(rest[j:], PlaceClose) {
			continue
		}
		n := 0
		for _, c := range rest[:j] {
			n = n*10 + int(c-'0')
		}
		return n, len(open) + j + len(PlaceClose), open, true
	}
	return 0, 0, "", false
}

// Placeholders returns every placeholder in a template, in order. The
// validator treats these exactly like markup tokens: the multiset must survive
// translation intact.
func Placeholders(template string) []string {
	var out []string
	for i := 0; i < len(template); {
		if _, width, _, matched := readPlaceholder(template[i:]); matched {
			out = append(out, template[i:i+width])
			i += width
			continue
		}
		i++
	}
	return out
}

// Stats summarizes an inventory in the terms that decide project size.
type Stats struct {
	TotalKeys     int `json:"total_keys"`
	UniqueValues  int `json:"unique_values"`
	Groups        int `json:"groups"`
	EmptyKeys     int `json:"empty_keys"`
	ReferenceKeys int `json:"reference_keys"`
	ProseKeys     int `json:"prose_keys"`
	// ProseGroups is the number that drives translation cost: distinct prose
	// templates, each needing exactly one translation call.
	ProseGroups     int `json:"prose_groups"`
	ReferenceGroups int `json:"reference_groups"`
	EmptyGroups     int `json:"empty_groups"`
	// ProseChars is the total rune count of the prose templates.
	ProseChars int `json:"prose_chars"`
	// MalformedKeys counts entries recovered from a source defect.
	MalformedKeys int `json:"malformed_keys"`
}

// Groups collapses entries into translation units, sorted by descending member
// count so the highest-leverage strings come first.
func Groups(entries []Entry) []Group {
	byTemplate := map[string]*Group{}
	for _, e := range entries {
		tmpl, toks, nums := Templatize(e.Value)
		g := byTemplate[tmpl]
		if g == nil {
			g = &Group{
				Template: tmpl,
				// Class is derived from the raw value: a placeholder itself
				// contains a letter and would otherwise read as prose.
				Class: Classify(e.Value),
				Rep:   e.Key,
			}
			byTemplate[tmpl] = g
		}
		if e.Key < g.Rep {
			g.Rep = e.Key
		}
		g.Members = append(g.Members, Member{Key: e.Key, Tokens: toks, Numbers: nums})
	}

	out := make([]Group, 0, len(byTemplate))
	for _, g := range byTemplate {
		sort.Slice(g.Members, func(i, j int) bool { return g.Members[i].Key < g.Members[j].Key })
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Members) != len(out[j].Members) {
			return len(out[i].Members) > len(out[j].Members)
		}
		return out[i].Template < out[j].Template
	})
	return out
}

// Summarize computes the headline numbers for a set of entries and groups.
func Summarize(entries []Entry, groups []Group) Stats {
	s := Stats{TotalKeys: len(entries)}

	uniq := map[string]struct{}{}
	for _, e := range entries {
		uniq[e.Value] = struct{}{}
		if e.Malformed {
			s.MalformedKeys++
		}
		switch Classify(e.Value) {
		case ClassEmpty:
			s.EmptyKeys++
		case ClassReference:
			s.ReferenceKeys++
		default:
			s.ProseKeys++
		}
	}
	s.UniqueValues = len(uniq)
	s.Groups = len(groups)

	for _, g := range groups {
		switch g.Class {
		case ClassEmpty:
			s.EmptyGroups++
		case ClassReference:
			s.ReferenceGroups++
		default:
			s.ProseGroups++
			s.ProseChars += len([]rune(g.Template))
		}
	}
	return s
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
