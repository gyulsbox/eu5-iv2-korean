package validate

import (
	"strings"
	"testing"
)

// hasRule reports whether findings contain the given rule.
func hasRule(findings []Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func TestCheckPairAcceptsSoundTranslations(t *testing.T) {
	cases := []struct{ name, key, src, dst string }{
		{
			"plain prose",
			"iv2_ideagroup_title_adm_1",
			"Innovativeness Ideas", "혁신 이념",
		},
		{
			"variables reordered for Korean word order",
			"iv2_msg",
			"$iv2_message_selected_ig$ $iv2_ideagroup_title_adm_1$",
			"$iv2_ideagroup_title_adm_1$$iv2_message_selected_ig$",
		},
		{
			"scope function kept",
			"iv2_appointed",
			"[SCOPE.sCharacter('recipient').GetName] was appointed.",
			"[SCOPE.sCharacter('recipient').GetName]이(가) 임명되었습니다.",
		},
		{
			"format pair and newline kept",
			"iv2_costs",
			`#bold Costs 25 @gold!#!\nto unlock.`,
			`#bold 25 @gold! 소요#!\n해금하려면.`,
		},
		{
			"empty source stays empty",
			"iv2_empty", "", "",
		},
		{
			"colour value copied verbatim",
			"iv2_idea_alert_adm_color", "green", "green",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			findings, ok := CheckPair(c.key, c.src, c.dst)
			if !ok {
				t.Errorf("rejected a sound translation: %+v", findings)
			}
		})
	}
}

func TestCheckPairCatchesMarkupDamage(t *testing.T) {
	const key = "iv2_costs"
	cases := []struct{ name, src, dst, rule string }{
		{
			"dropped variable",
			"Costs $GOLD$ gold", "금이 듭니다",
			RuleTokenMismatch,
		},
		{
			"renamed variable",
			"Costs $GOLD$ gold", "$금$ 비용",
			RuleTokenMismatch,
		},
		{
			"duplicated variable",
			"Costs $GOLD$ gold", "$GOLD$ $GOLD$ 비용",
			RuleTokenMismatch,
		},
		{
			"dropped format terminator leaks colour into the rest of the UI",
			`#bold Important#!`, `#bold 중요`,
			RuleTokenMismatch,
		},
		{
			"lost newline",
			`line one\nline two`, `첫 줄 둘째 줄`,
			RuleTokenMismatch,
		},
		{
			"mangled scope function",
			"[Country.GetName] rules", "[Country.이름] 통치",
			RuleTokenMismatch,
		},
		{
			"dropped text icon",
			"Costs @gold! to build", "건설 비용",
			RuleTokenMismatch,
		},
		{
			"blanked a non-empty string",
			"Innovativeness Ideas", "",
			RuleEmptyTranslation,
		},
		{
			"placeholder never substituted",
			"Level 1", "레벨 ⟦N1⟧",
			RuleLeakedPlaceholder,
		},
		{
			"token placeholder never substituted",
			"$a$ of $b$", "⟦T1⟧의 ⟦T2⟧",
			RuleLeakedPlaceholder,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			findings, ok := CheckPair(key, c.src, c.dst)
			if ok {
				t.Fatalf("accepted a broken translation")
			}
			if !hasRule(findings, c.rule) {
				t.Errorf("findings %+v, want rule %s", findings, c.rule)
			}
		})
	}
}

func TestCheckPairCatchesTranslatedEngineTokens(t *testing.T) {
	// Translating a colour name breaks the alert instead of merely reading
	// oddly, so it must be an error rather than a warning.
	findings, ok := CheckPair("iv2_idea_alert_adm_color", "green", "초록")
	if ok {
		t.Fatal("accepted a translated colour token")
	}
	if !hasRule(findings, RuleDoNotTranslate) {
		t.Errorf("findings %+v, want %s", findings, RuleDoNotTranslate)
	}
}

func TestFallbackKeepsEnglishOnFailure(t *testing.T) {
	const src = "Costs $GOLD$ gold"

	if got := Fallback("k", src, "$GOLD$ 비용"); got != "$GOLD$ 비용" {
		t.Errorf("sound translation was discarded: %q", got)
	}
	// This is the property the whole project rests on: a bad translation
	// degrades to English rather than shipping broken markup.
	if got := Fallback("k", src, "금이 듭니다"); got != src {
		t.Errorf("Fallback = %q, want the English source %q", got, src)
	}
}

func TestFindingCarriesBothStrings(t *testing.T) {
	findings, _ := CheckPair("k", "Costs $GOLD$", "비용")
	if len(findings) == 0 {
		t.Fatal("no findings")
	}
	f := findings[0]
	if f.Source != "Costs $GOLD$" || f.Translation != "비용" {
		t.Errorf("finding does not carry both strings: %+v", f)
	}
	if !strings.Contains(f.Message, "$GOLD$") {
		t.Errorf("message does not name the lost token: %q", f.Message)
	}
}

func TestDoNotTranslate(t *testing.T) {
	yes := []struct{ key, value string }{
		{"iv2_idea_alert_adm_color", "green"},
		{"iv2_idea_alert_color", "yellow"},
		{"Idea_Variation_2__iv2_alert_setting_1", "Idea_Variation_2__iv2_alert_setting_1"},
	}
	no := []struct{ key, value string }{
		{"iv2_ideagroup_title_adm_1", "Innovativeness Ideas"},
		// A one-word value that is ordinary English still needs translating;
		// only namespaced identifiers are engine tokens.
		{"iv2_limit_label", "Limit"},
		{"iv2_army_label", "Army"},
		{"iv2_msg", "$a$ $b$"},
	}
	for _, c := range yes {
		if !DoNotTranslate(c.key, c.value) {
			t.Errorf("DoNotTranslate(%q, %q) = false, want true", c.key, c.value)
		}
	}
	for _, c := range no {
		if DoNotTranslate(c.key, c.value) {
			t.Errorf("DoNotTranslate(%q, %q) = true, want false", c.key, c.value)
		}
	}
}

func TestASCIIPath(t *testing.T) {
	if !ASCIIPath(`C:/Users/me/Documents/Paradox Interactive/mod/iv2_korean`) {
		t.Error("ASCII path rejected")
	}
	// The exact failure mode called out in the brief: a Korean directory name
	// makes EU5 fail to load the mod.
	if ASCIIPath(`C:/Users/me/mod/한국어패치`) {
		t.Error("non-ASCII path accepted")
	}
}

func TestReportTallies(t *testing.T) {
	r := &Report{}
	r.add(Finding{Severity: Error, Rule: RuleTokenMismatch})
	r.add(Finding{Severity: Error, Rule: RuleTokenMismatch})
	r.add(Finding{Severity: Warn, Rule: RuleUntranslated})

	if r.Errors() != 2 {
		t.Errorf("Errors = %d, want 2", r.Errors())
	}
	if r.Warnings() != 1 {
		t.Errorf("Warnings = %d, want 1", r.Warnings())
	}
	byRule := r.ByRule()
	if len(byRule) != 2 || byRule[0].Rule != RuleTokenMismatch || byRule[0].Count != 2 {
		t.Errorf("ByRule = %+v", byRule)
	}
}
