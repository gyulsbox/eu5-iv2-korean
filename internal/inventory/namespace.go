package inventory

import (
	"regexp"
	"sort"
	"strings"
)

// modMarker matches the mod's own namespace anywhere in a key. IV2 keys are
// not all prefixed `iv2_`: the engine dictates prefixes like
// `STATIC_MODIFIER_NAME_`, `WE_PERFORM_`, and `game_concept_`, with the mod's
// namespace appearing mid-key.
var modMarker = regexp.MustCompile(`(?i)(^|_)(iv2|iv_2|idea_variation_2)(_|$)`)

// Namespaced reports whether a key carries the mod's own namespace somewhere.
// A key without it is a candidate base-game or foreign-mod key: redefining one
// is what crashed the previous Korean pack, so extract has to surface them for
// review rather than silently translating them.
func Namespaced(key string) bool {
	return modMarker.MatchString(key)
}

// PrefixCount is one row of the key-prefix histogram.
type PrefixCount struct {
	Prefix string `json:"prefix"`
	Count  int    `json:"count"`
}

// Prefixes builds a histogram over the first depth underscore-separated
// segments of each key, sorted by descending count.
func Prefixes(entries []Entry, depth int) []PrefixCount {
	counts := map[string]int{}
	for _, e := range entries {
		parts := strings.Split(e.Key, "_")
		if len(parts) > depth {
			parts = parts[:depth]
		}
		counts[strings.Join(parts, "_")]++
	}
	out := make([]PrefixCount, 0, len(counts))
	for p, c := range counts {
		out = append(out, PrefixCount{Prefix: p, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Prefix < out[j].Prefix
	})
	return out
}

// Foreign returns the entries whose keys carry no mod namespace, sorted by key.
func Foreign(entries []Entry) []Entry {
	var out []Entry
	for _, e := range entries {
		if !Namespaced(e.Key) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// ForeignShape collapses the un-namespaced keys into the small set of shapes a
// human can actually review. Raw counts run to thousands, but they come from a
// handful of engine prefixes: `STATIC_MODIFIER_NAME_…`, `WE_PERFORM_…` and so
// on. Every digit run in the key is folded to `#` so that numbered variants
// land in one row.
type ForeignShape struct {
	Shape   string `json:"shape"`
	Count   int    `json:"count"`
	Example string `json:"example"`
}

var digitRun = regexp.MustCompile(`[0-9]+`)

// ForeignShapes groups Foreign entries by key shape, most frequent first.
func ForeignShapes(entries []Entry) []ForeignShape {
	counts := map[string]int{}
	example := map[string]string{}
	for _, e := range Foreign(entries) {
		shape := digitRun.ReplaceAllString(e.Key, "#")
		counts[shape]++
		if _, ok := example[shape]; !ok {
			example[shape] = e.Key
		}
	}
	out := make([]ForeignShape, 0, len(counts))
	for s, c := range counts {
		out = append(out, ForeignShape{Shape: s, Count: c, Example: example[s]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Shape < out[j].Shape
	})
	return out
}
