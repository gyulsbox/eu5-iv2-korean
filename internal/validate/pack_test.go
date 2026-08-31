package validate

import (
	"os"
	"strings"
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
	// IV2 itself redefines two baseline keys, one in the same layer and one
	// across layers. Our pack must leave both alone.
	want := map[string]bool{"cw_settings_shared": true, "iv2_in_game_clash": true}
	if len(res.Shadowed) != len(want) {
		t.Fatalf("Shadowed = %v, want %d keys", res.Shadowed, len(want))
	}
	for _, k := range res.Shadowed {
		if !want[k] {
			t.Errorf("unexpected shadowed key %q", k)
		}
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

// The base game does not keep its localization in one place: it splits it
// across dlc/, in_game/, loading_screen/ and main_menu/, each with its own
// localization directory. A baseline scan that only looked at <root>/localization
// would find nothing and report a false pass, which is the most dangerous
// possible failure for this check.
func TestBaselineScanReachesEveryGameLayer(t *testing.T) {
	res, err := Run(Options{
		Source:    fixtureSource,
		Baselines: []string{fixtureBaseline},
	})
	if err != nil {
		t.Fatal(err)
	}

	wantLayers := map[string]string{
		"cw_settings_shared": "main_menu",
		"iv2_in_game_clash":  "in_game",
		"LOADING_TIP_1":      "loading_screen",
		"DLC_ONLY_KEY":       "dlc",
	}
	for key, layer := range wantLayers {
		ref, ok := res.BaselineKeys[key]
		if !ok {
			t.Errorf("baseline key %q not found; the scan missed the %s layer", key, layer)
			continue
		}
		if ref.Layer != layer {
			t.Errorf("%q layer = %q, want %q", key, ref.Layer, layer)
		}
	}
}

func TestShadowDetectionCrossesLayers(t *testing.T) {
	// IV2 defines iv2_in_game_clash in main_menu while the base game owns it
	// in in_game. A same-layer-only comparison would miss it.
	res, err := Run(Options{
		Source:    fixtureSource,
		Baselines: []string{fixtureBaseline},
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, k := range res.Shadowed {
		if k == "iv2_in_game_clash" {
			found = true
		}
	}
	if !found {
		t.Errorf("cross-layer collision missed; Shadowed = %v", res.Shadowed)
	}
	if got := res.BaselineKeys["iv2_in_game_clash"].Layer; got != "in_game" {
		t.Errorf("collision layer = %q, want in_game", got)
	}
}

func TestBaselineRefWhereNamesTheLayer(t *testing.T) {
	ref := BaselineRef{Root: "/eu5/game", File: "in_game/localization/english/gui_l_english.yml", Layer: "in_game"}
	if got := ref.Where(); !strings.Contains(got, "[in_game]") {
		t.Errorf("Where() = %q, want it to name the layer", got)
	}
}

func TestRunCatchesLayerMismatch(t *testing.T) {
	// IV2 defines this key under main_menu. A pack that files its translation
	// under in_game produces no error in game; the string is simply never
	// seen, which is worse than a loud failure.
	res, err := Run(Options{
		Source: fixtureSource,
		Out:    "testdata/out_wrong_layer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if countRule(res.Report, RuleLayerMismatch) != 1 {
		t.Errorf("layer-mismatch findings = %d, want 1", countRule(res.Report, RuleLayerMismatch))
	}
	if !keysWithRule(res.Report, RuleLayerMismatch)["iv2_ideagroup_title_adm_1"] {
		t.Error("wrong key reported for layer mismatch")
	}
}

func TestCorrectPackHasNoLayerMismatch(t *testing.T) {
	res, err := Run(Options{Source: fixtureSource, Out: fixtureGood})
	if err != nil {
		t.Fatal(err)
	}
	if n := countRule(res.Report, RuleLayerMismatch); n != 0 {
		t.Errorf("layer-mismatch findings = %d on a correct pack, want 0", n)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
