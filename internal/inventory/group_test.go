package inventory

import (
	"reflect"
	"testing"
)

func TestTemplatizeLiftsTokensAndNumbers(t *testing.T) {
	cases := []struct {
		value    string
		template string
		tokens   []string
		numbers  []string
	}{
		{
			value:    `$iv2_message_selected_ig$ $iv2_ideagroup_title_adm_1$`,
			template: `⟦T1⟧ ⟦T2⟧`,
			tokens:   []string{`$iv2_message_selected_ig$`, `$iv2_ideagroup_title_adm_1$`},
		},
		{
			value:    `This sets the basic Idea Group Limit to 4 per category.`,
			template: `This sets the basic Idea Group Limit to ⟦N1⟧ per category.`,
			numbers:  []string{"4"},
		},
		{
			value:    `Innovation Speed AI Bonus 25%`,
			template: `Innovation Speed AI Bonus ⟦N1⟧%`,
			numbers:  []string{"25"},
		},
		{
			// Digits inside a token belong to the token, not to the prose:
			// lifting the token first keeps `iv2` from becoming `iv⟦N1⟧`.
			value:    `Game Rule $setting_iv2_bonus_3$ gives 10%`,
			template: `Game Rule ⟦T1⟧ gives ⟦N1⟧%`,
			tokens:   []string{`$setting_iv2_bonus_3$`},
			numbers:  []string{"10"},
		},
		{
			value:    `plain text`,
			template: `plain text`,
		},
		{
			value:    ``,
			template: ``,
		},
	}
	for _, c := range cases {
		tmpl, toks, nums := Templatize(c.value)
		if tmpl != c.template {
			t.Errorf("Templatize(%q) template = %q, want %q", c.value, tmpl, c.template)
		}
		if !reflect.DeepEqual(toks, c.tokens) {
			t.Errorf("Templatize(%q) tokens = %v, want %v", c.value, toks, c.tokens)
		}
		if !reflect.DeepEqual(nums, c.numbers) {
			t.Errorf("Templatize(%q) numbers = %v, want %v", c.value, nums, c.numbers)
		}
	}
}

func TestTemplatizeDetemplatizeRoundTrip(t *testing.T) {
	values := []string{
		`$iv2_message_took_ig$ $iv2_ga_ideagroup_navy_9_3$ $IV_2_IDEA$.`,
		`Level 4 of 10`,
		`#bold Costs 25 £gold£#!\nand more`,
		`[SCOPE.sCharacter('recipient').GetName] was appointed as [iv2_researcher_adm|e].`,
		`no markup at all`,
		``,
		`123`,
		`⟦not a placeholder⟧`,
	}
	for _, v := range values {
		tmpl, toks, nums := Templatize(v)
		got, ok := Detemplatize(tmpl, toks, nums)
		if !ok {
			t.Errorf("Detemplatize reported a bad index for %q", v)
		}
		if got != v {
			t.Errorf("round trip of %q gave %q (template %q)", v, got, tmpl)
		}
	}
}

func TestDetemplatizeReordersPlaceholders(t *testing.T) {
	// Korean word order moves clauses; reconstruction must follow the index,
	// not the position.
	got, ok := Detemplatize(`⟦T2⟧를 ⟦T1⟧`, []string{`$verb$`, `$noun$`}, nil)
	if !ok {
		t.Fatal("Detemplatize reported failure")
	}
	if want := `$noun$를 $verb$`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDetemplatizeRejectsOutOfRange(t *testing.T) {
	// A translation that invented ⟦T3⟧ must not silently produce a broken
	// string; the caller falls back to English on false.
	if _, ok := Detemplatize(`⟦T1⟧ ⟦T3⟧`, []string{`$a$`}, nil); ok {
		t.Error("out-of-range token index reported ok")
	}
	if _, ok := Detemplatize(`⟦N2⟧`, nil, []string{"1"}); ok {
		t.Error("out-of-range number index reported ok")
	}
}

func TestPlaceholders(t *testing.T) {
	got := Placeholders(`⟦T1⟧ costs ⟦N1⟧ and ⟦T2⟧`)
	want := []string{`⟦T1⟧`, `⟦N1⟧`, `⟦T2⟧`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Placeholders = %v, want %v", got, want)
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]Class{
		``:                              ClassEmpty,
		`   `:                           ClassEmpty,
		`$a$ $b$`:                       ClassReference,
		`$a$ $b$.`:                      ClassReference,
		`[iv2_gc_ideagroup_cost_adm|e]`: ClassReference,
		`\n`:                            ClassReference,
		`OK`:                            ClassProse,
		`Costs $GOLD$ gold`:             ClassProse,
		`25%`:                           ClassReference,
	}
	for v, want := range cases {
		if got := Classify(v); got != want {
			t.Errorf("Classify(%q) = %s, want %s", v, got, want)
		}
	}
}

func TestGroupsCollapseNumericVariants(t *testing.T) {
	entries := []Entry{
		{Key: "iv2_ga_select_national_idea_adm_193_1", Value: "Level 1"},
		{Key: "iv2_ga_select_national_idea_adm_193_2", Value: "Level 2"},
		{Key: "iv2_ga_select_national_idea_adm_197_3", Value: "Level 3"},
		{Key: "other", Value: "Something else"},
	}
	groups := Groups(entries)
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(groups), groups)
	}
	if groups[0].Template != `Level ⟦N1⟧` || len(groups[0].Members) != 3 {
		t.Errorf("largest group = %q with %d members", groups[0].Template, len(groups[0].Members))
	}
	// The representative is deterministic so reruns produce stable output.
	if groups[0].Rep != "iv2_ga_select_national_idea_adm_193_1" {
		t.Errorf("rep = %q", groups[0].Rep)
	}
}

func TestGroupsCollapseReferenceChains(t *testing.T) {
	entries := []Entry{
		{Key: "a", Value: `$iv2_message_selected_ig$ $iv2_ideagroup_title_adm_1$`},
		{Key: "b", Value: `$iv2_message_selected_ig$ $iv2_ideagroup_title_adm_2$`},
		{Key: "c", Value: `$iv2_message_took_ig$ $iv2_ga_ideagroup_navy_9_3$`},
	}
	groups := Groups(entries)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1: %+v", len(groups), groups)
	}
	if groups[0].Class != ClassReference {
		t.Errorf("class = %s, want reference", groups[0].Class)
	}
}

func TestGroupMembersReconstructTheirOwnValue(t *testing.T) {
	entries := []Entry{
		{Key: "a", Value: `Level 1 of $max_4$`},
		{Key: "b", Value: `Level 9 of $max_7$`},
	}
	groups := Groups(entries)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	g := groups[0]
	byKey := map[string]string{"a": entries[0].Value, "b": entries[1].Value}
	for _, m := range g.Members {
		got, ok := Detemplatize(g.Template, m.Tokens, m.Numbers)
		if !ok || got != byKey[m.Key] {
			t.Errorf("member %s reconstructed as %q (ok=%v), want %q", m.Key, got, ok, byKey[m.Key])
		}
	}
}

func TestSummarize(t *testing.T) {
	entries := []Entry{
		{Key: "e", Value: ``},
		{Key: "r1", Value: `$a$`},
		{Key: "r2", Value: `$b$`},
		{Key: "p1", Value: `Level 1`},
		{Key: "p2", Value: `Level 2`},
		{Key: "p3", Value: `Distinct prose`},
		{Key: "bad", Value: `x`, Malformed: true},
	}
	s := Summarize(entries, Groups(entries))
	if s.TotalKeys != 7 {
		t.Errorf("TotalKeys = %d, want 7", s.TotalKeys)
	}
	if s.EmptyKeys != 1 || s.ReferenceKeys != 2 || s.ProseKeys != 4 {
		t.Errorf("class counts = %d/%d/%d, want 1/2/4", s.EmptyKeys, s.ReferenceKeys, s.ProseKeys)
	}
	// Level 1 and Level 2 collapse; x, Distinct prose stay separate.
	if s.ProseGroups != 3 {
		t.Errorf("ProseGroups = %d, want 3", s.ProseGroups)
	}
	if s.ReferenceGroups != 1 {
		t.Errorf("ReferenceGroups = %d, want 1", s.ReferenceGroups)
	}
	if s.MalformedKeys != 1 {
		t.Errorf("MalformedKeys = %d, want 1", s.MalformedKeys)
	}
}

func TestNamespaced(t *testing.T) {
	yes := []string{
		"iv2_ideagroup_title_adm_1",
		"WE_PERFORM_iv2_ga_select_ideagroup_adm_1_ACTION_LOG",
		"game_concept_iv2_researcher_desc",
		"Idea_Variation_2_name",
	}
	// Keys the mod owns but names without its own namespace. These are
	// indistinguishable from base-game keys by inspection alone, which is
	// exactly why extract has to surface them for review rather than
	// assuming they are safe to translate.
	no := []string{
		"cw_settings_something",
		"ADD_IDEAGROUP_SLOT_TOOLTIP_ADM",
		"languages_korean",
		"STATIC_MODIFIER_NAME_national_idea_modifier_adm_2",
	}
	for _, k := range yes {
		if !Namespaced(k) {
			t.Errorf("Namespaced(%q) = false, want true", k)
		}
	}
	for _, k := range no {
		if Namespaced(k) {
			t.Errorf("Namespaced(%q) = true, want false", k)
		}
	}
}
