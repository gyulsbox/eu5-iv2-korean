package validate

import (
	"testing"
)

const (
	fixtureSource   = "testdata/source"
	fixtureBaseline = "testdata/baseline/game"
	fixtureGood     = "testdata/out_good"
	fixtureBad      = "testdata/out_bad"
)

func countRule(rep *Report, rule string) int {
	n := 0
	for _, f := range rep.Findings {
		if f.Rule == rule {
			n++
		}
	}
	return n
}

func keysWithRule(rep *Report, rule string) map[string]bool {
	out := map[string]bool{}
	for _, f := range rep.Findings {
		if f.Rule == rule {
			out[f.Key] = true
		}
	}
	return out
}

func TestRunAcceptsACorrectPack(t *testing.T) {
	res, err := Run(Options{
		Source:    fixtureSource,
		Out:       fixtureGood,
		Baselines: []string{fixtureBaseline},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := res.Report.Errors(); n != 0 {
		t.Errorf("got %d errors on a correct pack:", n)
		for _, f := range res.Report.Findings {
			if f.Severity == Error {
				t.Errorf("  %s %s: %s", f.Rule, f.Key, f.Message)
			}
		}
	}
	if len(res.Report.FallbackKeys) != 0 {
		t.Errorf("FallbackKeys = %v, want none", res.Report.FallbackKeys)
	}
}

func TestRunFlagsBaselineCollisionAsUnproven(t *testing.T) {
	// Without a baseline the collision guarantee is unproven. Reporting a
	// clean pass here would repeat exactly the assumption that crashed the
	// previous Korean pack.
	res, err := Run(Options{Source: fixtureSource, Out: fixtureGood})
	if err != nil {
		t.Fatal(err)
	}
	if res.Report.BaselineChecked {
		t.Error("BaselineChecked true with no baseline supplied")
	}
	if len(res.Shadowed) != 0 {
		t.Errorf("Shadowed = %v without a baseline, want none", res.Shadowed)
	}
}

func TestRunFindsShadowedSourceKeys(t *testing.T) {
	res, err := Run(Options{
		Source:    fixtureSource,
		Baselines: []string{fixtureBaseline},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Report.BaselineChecked {
		t.Error("BaselineChecked false with a baseline supplied")
	}
	// IV2 itself redefines cw_settings_shared. Our pack must leave it alone.
	if len(res.Shadowed) != 1 || res.Shadowed[0] != "cw_settings_shared" {
		t.Errorf("Shadowed = %v, want [cw_settings_shared]", res.Shadowed)
	}
}

func TestRunCatchesEveryFailureMode(t *testing.T) {
	res, err := Run(Options{
		Source:    fixtureSource,
		Out:       fixtureBad,
		Baselines: []string{fixtureBaseline},
	})
	if err != nil {
		t.Fatal(err)
	}
	rep := res.Report

	for _, rule := range []string{
		RuleTokenMismatch,
		RuleEmptyTranslation,
		RuleLeakedPlaceholder,
		RuleDoNotTranslate,
		RuleUnknownKey,
		RuleShadowsBaseline,
		RuleMissingBOM,
		RuleBadHeader,
		RuleBadFilename,
		RuleReplaceDir,
		RuleUntranslated,
	} {
		if countRule(rep, rule) == 0 {
			t.Errorf("rule %s produced no findings", rule)
		}
	}

	// The specific keys matter, not just the counts.
	mismatched := keysWithRule(rep, RuleTokenMismatch)
	if !mismatched["iv2_ideagroup_title_adm_1"] {
		t.Error("invented $LOST$ variable not caught")
	}
	if !mismatched["iv2_costs_tt"] {
		t.Error("dropped @gold! and #! not caught")
	}
	if !keysWithRule(rep, RuleUnknownKey)["iv2_invented_key"] {
		t.Error("key absent from the source was not caught")
	}
	if !keysWithRule(rep, RuleShadowsBaseline)["cw_settings_shared"] {
		t.Error("base-game key redefinition was not caught")
	}
	if !keysWithRule(rep, RuleDoNotTranslate)["iv2_idea_alert_adm_color"] {
		t.Error("translated colour token was not caught")
	}
	// iv2_msg was copied verbatim from English, which is reference-only and
	// therefore fine; it must not be reported as untranslated prose.
	if keysWithRule(rep, RuleUntranslated)["iv2_msg"] {
		t.Error("reference-only value reported as untranslated")
	}

	// Every failing key must be queued for English fallback.
	for _, k := range []string{
		"iv2_ideagroup_title_adm_1", "iv2_costs_tt",
		"iv2_desc", "iv2_level_1", "iv2_idea_alert_adm_color",
	} {
		found := false
		for _, fk := range rep.FallbackKeys {
			if fk == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s not queued for English fallback; FallbackKeys = %v", k, rep.FallbackKeys)
		}
	}
}

func TestRunReportsMalformedSource(t *testing.T) {
	res, err := Run(Options{Source: fixtureSource})
	if err != nil {
		t.Fatal(err)
	}
	if countRule(res.Report, RuleMalformedSource) != 1 {
		t.Errorf("malformed-source findings = %d, want 1", countRule(res.Report, RuleMalformedSource))
	}
	// A source defect is upstream's, not ours: it must not block our build.
	for _, f := range res.Report.Findings {
		if f.Rule == RuleMalformedSource && f.Severity != Warn {
			t.Errorf("malformed-source severity = %s, want warn", f.Severity)
		}
	}
}

func TestRunListsDoNotTranslateKeys(t *testing.T) {
	res, err := Run(Options{Source: fixtureSource})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.DoNotTranslateKeys) != 1 || res.DoNotTranslateKeys[0] != "iv2_idea_alert_adm_color" {
		t.Errorf("DoNotTranslateKeys = %v, want [iv2_idea_alert_adm_color]", res.DoNotTranslateKeys)
	}
}

func TestRunRejectsMissingSource(t *testing.T) {
	if _, err := Run(Options{Source: "testdata/does_not_exist"}); err == nil {
		t.Error("Run accepted a missing source directory")
	}
}
