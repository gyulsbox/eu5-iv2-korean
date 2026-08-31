package paradox

import (
	"strings"
	"testing"
)

func TestParseBasic(t *testing.T) {
	src := BOM + `l_english:
 simple: "Hello"
 versioned:0 "With version"
 versioned_big:12 "Version twelve"
 no_space:0"Tight"
 empty: ""
`
	f, errs := ParseString(src)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !f.HadBOM {
		t.Error("BOM not detected")
	}
	if f.Language != "l_english" {
		t.Errorf("language = %q, want l_english", f.Language)
	}

	want := []Entry{
		{Key: "simple", Version: -1, Value: "Hello"},
		{Key: "versioned", Version: 0, Value: "With version"},
		{Key: "versioned_big", Version: 12, Value: "Version twelve"},
		{Key: "no_space", Version: 0, Value: "Tight"},
		{Key: "empty", Version: -1, Value: ""},
	}
	if len(f.Entries) != len(want) {
		t.Fatalf("got %d entries, want %d", len(f.Entries), len(want))
	}
	for i, w := range want {
		g := f.Entries[i]
		if g.Key != w.Key || g.Version != w.Version || g.Value != w.Value {
			t.Errorf("entry %d = %+v, want key=%q version=%d value=%q",
				i, g, w.Key, w.Version, w.Value)
		}
	}
}

func TestParseCommentsVsFormatTokens(t *testing.T) {
	// A `#` inside a quoted value is a format token, not a comment. Getting
	// this wrong truncates every coloured string in the mod.
	src := `l_english:
 # a whole-line comment
 ## a doubled comment
 colored: "#bold Important#! text"    # trailing comment
 hash_only: "cost is 100# of base"
 after_close: "value" # comment "with quotes"
`
	f, errs := ParseString(src)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	cases := map[string]string{
		"colored":     "#bold Important#! text",
		"hash_only":   "cost is 100# of base",
		"after_close": "value",
	}
	if len(f.Entries) != len(cases) {
		t.Fatalf("got %d entries, want %d: %+v", len(f.Entries), len(cases), f.Entries)
	}
	for _, e := range f.Entries {
		if want, ok := cases[e.Key]; !ok {
			t.Errorf("unexpected key %q", e.Key)
		} else if e.Value != want {
			t.Errorf("%s = %q, want %q", e.Key, e.Value, want)
		}
	}
}

func TestParseEscapes(t *testing.T) {
	src := `l_english:
 quoted: "He said \"yes\" loudly"
 newline: "line one\nline two"
 backslash: "path\\to\\thing"
 tab: "a\tb"
`
	f, errs := ParseString(src)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	want := map[string]string{
		"quoted": `He said "yes" loudly`,
		// \n must survive as the two-character token the validator counts,
		// not as a real line break.
		"newline": `line one\nline two`,
		// Backslash sequences other than \" are preserved exactly, so the
		// value re-emits byte for byte.
		"backslash": `path\\to\\thing`,
		"tab":       `a\tb`,
	}
	for _, e := range f.Entries {
		if e.Value != want[e.Key] {
			t.Errorf("%s = %q, want %q", e.Key, e.Value, want[e.Key])
		}
	}
}

func TestParseWhitespaceTolerance(t *testing.T) {
	src := "l_english:\n\n\t\tindented: \"tabs\"\n      spaced:  \"spaces\"\n\n \n  trailing: \"ok\"   \n"
	f, errs := ParseString(src)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(f.Entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(f.Entries), f.Entries)
	}
}

func TestParseRecoversUnterminatedValue(t *testing.T) {
	// IV2 ships eight of these. Dropping the key would silently lose
	// translatable text, so the parser recovers at end of line and flags it.
	src := `l_english:
 broken: "[Scope.GetName] was appointed.
 fine: "next key still parses"
`
	f, errs := ParseString(src)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
	if len(f.Entries) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(f.Entries), f.Entries)
	}
	if !f.Entries[0].Malformed {
		t.Error("recovered entry not flagged as malformed")
	}
	if got := f.Entries[0].Value; got != "[Scope.GetName] was appointed." {
		t.Errorf("recovered value = %q", got)
	}
	if f.Entries[1].Malformed {
		t.Error("following entry wrongly flagged malformed")
	}
}

func TestParseNoBOM(t *testing.T) {
	f, _ := ParseString("l_english:\n key: \"v\"\n")
	if f.HadBOM {
		t.Error("HadBOM true without a BOM")
	}
}

func TestParseRejectsUnquotedValue(t *testing.T) {
	f, errs := ParseString("l_english:\n key: bare\n")
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errs))
	}
	if len(f.Entries) != 0 {
		t.Errorf("got %d entries, want 0", len(f.Entries))
	}
}

func TestQuoteRoundTrip(t *testing.T) {
	values := []string{
		`plain`,
		`with "quotes" inside`,
		`line\nbreak`,
		`#bold x#!`,
		`$VAR$ and [Scope.Get] and £gold£ and @icon!`,
		``,
	}
	for _, v := range values {
		src := "l_english:\n key: " + Quote(v) + "\n"
		f, errs := ParseString(src)
		if len(errs) != 0 {
			t.Fatalf("%q: parse errors %v (source %s)", v, errs, src)
		}
		if len(f.Entries) != 1 {
			t.Fatalf("%q: got %d entries", v, len(f.Entries))
		}
		if got := f.Entries[0].Value; got != v {
			t.Errorf("round trip of %q gave %q", v, got)
		}
	}
}

func TestLookup(t *testing.T) {
	f, _ := ParseString("l_english:\n a: \"1\"\n b: \"2\"\n")
	if e, ok := f.Lookup("b"); !ok || e.Value != "2" {
		t.Errorf("Lookup(b) = %+v, %v", e, ok)
	}
	if _, ok := f.Lookup("missing"); ok {
		t.Error("Lookup(missing) reported found")
	}
}

func TestParseLongValue(t *testing.T) {
	long := strings.Repeat("x", 200000)
	f, errs := ParseString("l_english:\n k: \"" + long + "\"\n")
	if len(errs) != 0 {
		t.Fatalf("errors on long value: %v", errs)
	}
	if len(f.Entries) != 1 || len(f.Entries[0].Value) != len(long) {
		t.Error("long value not parsed intact")
	}
}
