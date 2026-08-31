// Package validate enforces the invariants that keep a generated Korean pack
// from crashing Europa Universalis V.
//
// Two failures killed the previous community pack, and both are checked here.
// The first is markup damage: a translation that drops or renames a $VAR$ or
// [Scope.Func] leaves a tooltip that cannot render. The second, and the fatal
// one, is scope creep: a pack that defines keys the base game or another mod
// already owns. Everything in this package exists to make those two states
// unreachable rather than merely unlikely.
package validate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/inventory"
	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/paradox"
)

// Severity separates findings that must block a build from findings a human
// should look at.
type Severity string

const (
	// Error means the pack is not safe to ship as it stands. For a
	// translation defect the offending key falls back to English; for a
	// structural defect the build itself is wrong.
	Error Severity = "error"
	// Warn means something worth a look that does not endanger the game.
	Warn Severity = "warn"
)

// Rule names the invariant a finding violates. They are stable strings so
// reports can be diffed between runs.
const (
	RuleTokenMismatch     = "token-mismatch"
	RuleLeakedPlaceholder = "leaked-placeholder"
	RuleEmptyTranslation  = "empty-translation"
	RuleUnknownKey        = "unknown-key"
	RuleShadowsBaseline   = "shadows-baseline"
	RuleLayerMismatch     = "layer-mismatch"
	RuleDoNotTranslate    = "do-not-translate"
	RuleMissingBOM        = "missing-bom"
	RuleBadHeader         = "bad-header"
	RuleBadFilename       = "bad-filename"
	RuleReplaceDir        = "replace-dir"
	RuleDuplicateKey      = "duplicate-key"
	RuleNonASCIIPath      = "non-ascii-path"
	RuleUntranslated      = "untranslated"
	RuleMalformedSource   = "malformed-source"
)

// Finding is one violation.
type Finding struct {
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	Key      string   `json:"key,omitempty"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
	Message  string   `json:"message"`
	// Source and Translation are carried so a report is actionable without
	// cross-referencing the files by hand.
	Source      string `json:"source,omitempty"`
	Translation string `json:"translation,omitempty"`
}

// Report collects findings from a validation run.
type Report struct {
	Findings []Finding `json:"findings"`
	// FallbackKeys lists keys whose translation failed validation and must
	// therefore ship as English. Shipping the wrong language is survivable;
	// shipping broken markup is not.
	FallbackKeys []string `json:"fallback_keys,omitempty"`
	// BaselineChecked records whether a baseline was supplied. Without one,
	// the "no base-game keys" guarantee is unproven rather than satisfied,
	// and the report must not imply otherwise.
	BaselineChecked bool `json:"baseline_checked"`
}

func (r *Report) add(f Finding) { r.Findings = append(r.Findings, f) }

// Errors returns the number of blocking findings.
func (r *Report) Errors() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == Error {
			n++
		}
	}
	return n
}

// Warnings returns the number of non-blocking findings.
func (r *Report) Warnings() int { return len(r.Findings) - r.Errors() }

// ByRule tallies findings per rule, most frequent first.
func (r *Report) ByRule() []RuleCount {
	counts := map[string]RuleCount{}
	for _, f := range r.Findings {
		c := counts[f.Rule]
		c.Rule = f.Rule
		c.Count++
		if f.Severity == Error {
			c.Severity = Error
		} else if c.Severity == "" {
			c.Severity = Warn
		}
		counts[f.Rule] = c
	}
	out := make([]RuleCount, 0, len(counts))
	for _, c := range counts {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

// RuleCount is one row of a report summary.
type RuleCount struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Count    int      `json:"count"`
}

// CheckPair applies every per-string invariant to one source/translation pair
// and reports whether the translation is safe to ship.
//
// The token multiset equality at its centre is the assertion the whole project
// rests on: if it holds, a mistranslation reads badly but the game still runs.
func CheckPair(key, source, translation string) (findings []Finding, ok bool) {
	ok = true

	// A placeholder that survived into the output means the build stage failed
	// to substitute it. The game would render the literal ⟦T1⟧.
	if leaked := inventory.Placeholders(translation); len(leaked) > 0 {
		findings = append(findings, Finding{
			Severity: Error, Rule: RuleLeakedPlaceholder, Key: key,
			Message: fmt.Sprintf("translation still contains %s",
				strings.Join(leaked, " ")),
			Source: source, Translation: translation,
		})
		ok = false
	}

	// Dropping the text of a non-empty string blanks the UI element.
	if strings.TrimSpace(source) != "" && strings.TrimSpace(translation) == "" {
		findings = append(findings, Finding{
			Severity: Error, Rule: RuleEmptyTranslation, Key: key,
			Message: "non-empty source translated to an empty string",
			Source:  source, Translation: translation,
		})
		ok = false
	}

	missing, added := paradox.TokenDiff(source, translation)
	if len(missing) > 0 || len(added) > 0 {
		findings = append(findings, Finding{
			Severity: Error, Rule: RuleTokenMismatch, Key: key,
			Message: fmt.Sprintf("markup differs: %s",
				describeDiff(missing, added)),
			Source: source, Translation: translation,
		})
		ok = false
	}

	// Keys whose value the engine reads as data rather than as display text.
	// Translating iv2_idea_alert_adm_color from "green" breaks the alert
	// colour instead of merely reading oddly.
	if DoNotTranslate(key, source) && translation != source {
		findings = append(findings, Finding{
			Severity: Error, Rule: RuleDoNotTranslate, Key: key,
			Message: "value is an engine token, not display text; it must be copied verbatim",
			Source:  source, Translation: translation,
		})
		ok = false
	}

	return findings, ok
}

func describeDiff(missing, added map[string]int) string {
	var parts []string
	if s := formatMultiset(missing); s != "" {
		parts = append(parts, "lost "+s)
	}
	if s := formatMultiset(added); s != "" {
		parts = append(parts, "gained "+s)
	}
	return strings.Join(parts, ", ")
}

func formatMultiset(m map[string]int) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if m[k] > 1 {
			parts = append(parts, fmt.Sprintf("%s x%d", k, m[k]))
		} else {
			parts = append(parts, k)
		}
	}
	return strings.Join(parts, " ")
}

// Fallback returns the string that should actually be written for a key. A
// translation that fails validation is replaced by its English source, which
// is the guarantee that a bad translation can never crash the game.
func Fallback(key, source, translation string) string {
	if _, ok := CheckPair(key, source, translation); ok {
		return translation
	}
	return source
}

// identifierValue matches a value that is a bare identifier rather than
// prose: no spaces, and shaped like a script token.
var identifierValue = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// DoNotTranslate reports whether a key's value is consumed by the engine
// rather than shown to the player.
//
// Two shapes occur in IV2. Keys ending in `_color` hold a colour name the
// engine looks up ("green", "yellow"). Keys whose value is an identifier
// carrying the mod's own namespace are cross-references, not text.
func DoNotTranslate(key, value string) bool {
	if strings.HasSuffix(key, "_color") {
		return true
	}
	if identifierValue.MatchString(value) && inventory.Namespaced(value) {
		return true
	}
	return false
}

// ASCIIPath reports whether a path is pure ASCII. EU5 fails to load mods from
// paths containing non-ASCII characters, which is why the mod directory name
// must never be Korean.
func ASCIIPath(p string) bool {
	for _, r := range p {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}
