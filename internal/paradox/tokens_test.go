package paradox

import (
	"reflect"
	"testing"
)

func TestTokensKinds(t *testing.T) {
	cases := []struct {
		value string
		want  []Token
	}{
		{`$VALUE$`, []Token{{Kind: KindVariable, Text: `$VALUE$`}}},
		{`[Country.GetName]`, []Token{{Kind: KindScope, Text: `[Country.GetName]`}}},
		{`[iv2_mod|e]`, []Token{{Kind: KindScope, Text: `[iv2_mod|e]`}}},
		{`[SCOPE.sCharacter('recipient').GetName]`,
			[]Token{{Kind: KindScope, Text: `[SCOPE.sCharacter('recipient').GetName]`}}},
		{`£gold£`, []Token{{Kind: KindIcon, Text: `£gold£`}}},
		{`@gold!`, []Token{{Kind: KindTextIcon, Text: `@gold!`}}},
		{`\n`, []Token{{Kind: KindNewline, Text: `\n`}}},
		{`#bold x#!`, []Token{
			{Kind: KindFormat, Text: `#bold`},
			{Kind: KindFormatOff, Text: `#!`},
		}},
		{`#R;bold x#!`, []Token{
			{Kind: KindFormat, Text: `#R;bold`},
			{Kind: KindFormatOff, Text: `#!`},
		}},
	}
	for _, c := range cases {
		got := Tokens(c.value)
		if len(got) != len(c.want) {
			t.Errorf("Tokens(%q) returned %d tokens, want %d: %+v",
				c.value, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i].Kind != c.want[i].Kind || got[i].Text != c.want[i].Text {
				t.Errorf("Tokens(%q)[%d] = {%s %q}, want {%s %q}",
					c.value, i, got[i].Kind, got[i].Text, c.want[i].Kind, c.want[i].Text)
			}
		}
	}
}

func TestTokensNoFalsePositives(t *testing.T) {
	// A bare number after a hash, and prose punctuation, must not be read as
	// markup: treating them as tokens would make the validator reject sound
	// translations.
	for _, v := range []string{
		"costs 100# of base",
		"email me at name@example",
		"a plain sentence.",
		"5 > 3 and 2 < 4",
	} {
		if got := Tokens(v); len(got) != 0 {
			t.Errorf("Tokens(%q) = %+v, want none", v, got)
		}
	}
}

func TestTokensOrderAndOffsets(t *testing.T) {
	v := `Costs $A$ and [B.C] then \n £d£`
	got := Tokens(v)
	wantText := []string{`$A$`, `[B.C]`, `\n`, `£d£`}
	for i, w := range wantText {
		if i >= len(got) {
			t.Fatalf("got %d tokens, want %d", len(got), len(wantText))
		}
		if got[i].Text != w {
			t.Errorf("token %d = %q, want %q", i, got[i].Text, w)
		}
		if v[got[i].Start:got[i].End] != w {
			t.Errorf("token %d offsets do not slice back to %q", i, w)
		}
	}
}

func TestTokenMultisetAndSignature(t *testing.T) {
	v := `$A$ $A$ [B]`
	want := map[string]int{`$A$`: 2, `[B]`: 1}
	if got := TokenMultiset(v); !reflect.DeepEqual(got, want) {
		t.Errorf("TokenMultiset(%q) = %v, want %v", v, got, want)
	}
	// Reordering must not change the signature; that is what lets a Korean
	// translation move clauses around.
	if TokenSignature(`$A$ [B] $A$`) != TokenSignature(v) {
		t.Error("signature changed under token reordering")
	}
	if TokenSignature(`$A$ [B]`) == TokenSignature(v) {
		t.Error("signature ignored a dropped token")
	}
}

func TestTokenDiff(t *testing.T) {
	src := `Costs $GOLD$ £gold£ and \n breaks`

	t.Run("intact translation passes", func(t *testing.T) {
		missing, added := TokenDiff(src, `\n £gold£ 비용은 $GOLD$ 입니다`)
		if len(missing) != 0 || len(added) != 0 {
			t.Errorf("missing=%v added=%v, want both empty", missing, added)
		}
	})

	t.Run("dropped token is caught", func(t *testing.T) {
		missing, added := TokenDiff(src, `비용은 $GOLD$ \n 입니다`)
		if missing[`£gold£`] != 1 {
			t.Errorf("missing = %v, want £gold£:1", missing)
		}
		if len(added) != 0 {
			t.Errorf("added = %v, want empty", added)
		}
	})

	t.Run("mangled token is caught", func(t *testing.T) {
		// The classic failure: a translator renames the variable.
		missing, added := TokenDiff(src, `$금$ £gold£ \n`)
		if missing[`$GOLD$`] != 1 {
			t.Errorf("missing = %v, want $GOLD$:1", missing)
		}
		if added[`$금$`] != 1 {
			t.Errorf("added = %v, want $금$:1", added)
		}
	})

	t.Run("duplicated token is caught", func(t *testing.T) {
		missing, added := TokenDiff(src, `$GOLD$ $GOLD$ £gold£ \n`)
		if added[`$GOLD$`] != 1 {
			t.Errorf("added = %v, want $GOLD$:1", added)
		}
		if len(missing) != 0 {
			t.Errorf("missing = %v, want empty", missing)
		}
	})

	t.Run("lost newline is caught", func(t *testing.T) {
		missing, _ := TokenDiff(src, `$GOLD$ £gold£ 줄바꿈 없음`)
		if missing[`\n`] != 1 {
			t.Errorf("missing = %v, want \\n:1", missing)
		}
	})
}
