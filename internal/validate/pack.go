package validate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/inventory"
)

// Options configures a validation run.
type Options struct {
	// Source is the IV2 mod directory, the authority on which keys exist.
	Source string
	// Out is our generated Korean pack. When empty, only source-side
	// analysis runs.
	Out string
	// Baselines are localization trees we must never redefine: the base
	// game, and any other mod in the load order (FUM, Glorp UI, CMM).
	Baselines []string

	SourceLang string
	TargetLang string
}

func (o Options) sourceLang() string {
	if o.SourceLang == "" {
		return "english"
	}
	return o.SourceLang
}

func (o Options) targetLang() string {
	if o.TargetLang == "" {
		return "korean"
	}
	return o.TargetLang
}

// Result carries a report together with the inventories it was computed from,
// so callers can print counts without rescanning.
type Result struct {
	Report *Report
	// Source is the IV2 inventory.
	Source *inventory.Inventory
	// Out is our pack's inventory, nil when no output was given.
	Out *inventory.Inventory
	// BaselineKeys is the set of keys owned by the base game or other mods.
	BaselineKeys map[string]BaselineRef
	// Shadowed lists IV2's own keys that also exist in a baseline. These are
	// the keys our pack must leave alone: translating one means overriding
	// somebody else's localization, which is what crashed the last pack.
	Shadowed []string
	// DoNotTranslateKeys lists source keys whose value the engine consumes.
	DoNotTranslateKeys []string
}

// BaselineRef records where a baseline key was found. The layer matters:
// EU5 splits localization across dlc, in_game, loading_screen and main_menu,
// and a collision inside the same layer is unambiguous while one across
// layers may or may not bite depending on when each is loaded. Both are
// reported as errors, but the layer is carried so a human can tell them apart.
type BaselineRef struct {
	Root  string `json:"root"`
	File  string `json:"file"`
	Layer string `json:"layer"`
}

// Where renders a reference for a message.
func (b BaselineRef) Where() string {
	if b.Layer != "" && b.Layer != "." {
		return b.Root + " [" + b.Layer + "] " + b.File
	}
	return b.Root + " " + b.File
}

// Run performs a full validation pass.
func Run(o Options) (*Result, error) {
	res := &Result{Report: &Report{}, BaselineKeys: map[string]BaselineRef{}}
	rep := res.Report

	src, err := inventory.Scan(o.Source, o.sourceLang())
	if err != nil {
		return nil, fmt.Errorf("scanning source: %w", err)
	}
	if len(src.Files) == 0 {
		return nil, fmt.Errorf("no *_l_%s.yml files found under %s", o.sourceLang(), o.Source)
	}
	res.Source = src

	srcValue := make(map[string]string, len(src.Entries))
	srcEntry := make(map[string]inventory.Entry, len(src.Entries))
	for _, e := range src.Entries {
		if _, seen := srcValue[e.Key]; !seen {
			srcValue[e.Key] = e.Value
			srcEntry[e.Key] = e
		}
		if e.Malformed {
			rep.add(Finding{
				Severity: Warn, Rule: RuleMalformedSource, Key: e.Key,
				File: e.File, Line: e.Line,
				Message: "source value has no closing quote; recovered at end of line",
				Source:  e.Value,
			})
		}
		if DoNotTranslate(e.Key, e.Value) {
			res.DoNotTranslateKeys = append(res.DoNotTranslateKeys, e.Key)
		}
	}
	sort.Strings(res.DoNotTranslateKeys)

	// Baselines are scanned in both languages: a key the base game defines in
	// either one is a key we must not touch.
	for _, b := range o.Baselines {
		for _, lang := range []string{o.sourceLang(), o.targetLang()} {
			inv, err := inventory.Scan(b, lang)
			if err != nil {
				return nil, fmt.Errorf("scanning baseline %s: %w", b, err)
			}
			for _, e := range inv.Entries {
				if _, seen := res.BaselineKeys[e.Key]; !seen {
					res.BaselineKeys[e.Key] = BaselineRef{
						Root: b, File: e.File, Layer: e.Layer,
					}
				}
			}
		}
	}
	rep.BaselineChecked = len(o.Baselines) > 0

	for key := range srcValue {
		if _, clash := res.BaselineKeys[key]; clash {
			res.Shadowed = append(res.Shadowed, key)
		}
	}
	sort.Strings(res.Shadowed)

	if o.Out == "" {
		return res, nil
	}

	if err := checkOutputLayout(o, rep); err != nil {
		return nil, err
	}

	out, err := inventory.Scan(o.Out, o.targetLang())
	if err != nil {
		return nil, fmt.Errorf("scanning output: %w", err)
	}
	res.Out = out

	wantHeader := "l_" + o.targetLang()
	for _, f := range out.Files {
		if !f.HadBOM {
			rep.add(Finding{
				Severity: Error, Rule: RuleMissingBOM, File: f.Path,
				Message: "file has no UTF-8 BOM; EU5 ignores it silently",
			})
		}
		if f.Language != wantHeader {
			rep.add(Finding{
				Severity: Error, Rule: RuleBadHeader, File: f.Path,
				Message: fmt.Sprintf("header is %q, want %q", f.Language, wantHeader),
			})
		}
		if !strings.Contains(f.Path, "/localization/"+o.targetLang()+"/") {
			rep.add(Finding{
				Severity: Error, Rule: RuleBadFilename, File: f.Path,
				Message: "file is not under a localization/" + o.targetLang() + " directory",
			})
		}
	}

	for key, locs := range out.Duplicates {
		rep.add(Finding{
			Severity: Warn, Rule: RuleDuplicateKey, Key: key,
			Message: "defined more than once: " + strings.Join(locs, ", "),
		})
	}

	fallback := map[string]struct{}{}
	for _, e := range out.Entries {
		if e.Replace {
			rep.add(Finding{
				Severity: Error, Rule: RuleReplaceDir, Key: e.Key, File: e.File,
				Message: "key sits under replace/, which overrides other mods; " +
					"we only define IV2's own keys and must never need it",
			})
		}

		source, known := srcValue[e.Key]
		if !known {
			rep.add(Finding{
				Severity: Error, Rule: RuleUnknownKey, Key: e.Key,
				File: e.File, Line: e.Line,
				Message:     "key is not defined by IV2; the pack must not invent keys",
				Translation: e.Value,
			})
			continue
		}

		if ref, clash := res.BaselineKeys[e.Key]; clash {
			rep.add(Finding{
				Severity: Error, Rule: RuleShadowsBaseline, Key: e.Key,
				File: e.File, Line: e.Line,
				Message: "key is also defined by " + ref.Where() +
					"; translating it overrides their localization",
				Translation: e.Value,
			})
		}

		// Our pack must define a key in the same layer IV2 does. EU5 loads
		// dlc, in_game, loading_screen and main_menu separately, so a
		// translation filed under the wrong one is simply never seen.
		if srcE := srcEntry[e.Key]; srcE.Layer != "" && srcE.Layer != e.Layer {
			rep.add(Finding{
				Severity: Error, Rule: RuleLayerMismatch, Key: e.Key,
				File: e.File, Line: e.Line,
				Message: fmt.Sprintf("IV2 defines this key in %s but the pack puts it in %s; "+
					"EU5 loads those layers separately", srcE.Layer, e.Layer),
			})
		}

		findings, ok := CheckPair(e.Key, source, e.Value)
		for _, f := range findings {
			f.File, f.Line = e.File, e.Line
			rep.add(f)
		}
		if !ok {
			fallback[e.Key] = struct{}{}
			continue
		}

		if e.Value == source && inventory.Classify(source) == inventory.ClassProse &&
			!DoNotTranslate(e.Key, source) {
			rep.add(Finding{
				Severity: Warn, Rule: RuleUntranslated, Key: e.Key,
				File: e.File, Line: e.Line,
				Message: "value is identical to the English source",
				Source:  source,
			})
		}
	}

	for k := range fallback {
		rep.FallbackKeys = append(rep.FallbackKeys, k)
	}
	sort.Strings(rep.FallbackKeys)

	return res, nil
}

// checkOutputLayout inspects the output tree itself rather than its contents:
// the directory name's character set, and any .yml that the loader will skip
// because it is misnamed.
func checkOutputLayout(o Options, rep *Report) error {
	abs, err := filepath.Abs(o.Out)
	if err != nil {
		abs = o.Out
	}
	if !ASCIIPath(abs) {
		rep.add(Finding{
			Severity: Error, Rule: RuleNonASCIIPath, File: abs,
			Message: "mod path contains non-ASCII characters; EU5 fails to load it",
		})
	}

	suffix := "_l_" + o.targetLang() + ".yml"
	return filepath.WalkDir(o.Out, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".git" || name == ".metadata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".yml") || strings.HasSuffix(d.Name(), suffix) {
			return nil
		}
		rel, relErr := filepath.Rel(o.Out, p)
		if relErr != nil {
			rel = p
		}
		rep.add(Finding{
			Severity: Error, Rule: RuleBadFilename, File: filepath.ToSlash(rel),
			Message: "localization file must end in " + suffix + " or the game will not load it",
		})
		return nil
	})
}
