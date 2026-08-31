package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/validate"
)

// repeatedFlag collects a flag that may be given more than once.
type repeatedFlag []string

func (r *repeatedFlag) String() string { return strings.Join(*r, ",") }

func (r *repeatedFlag) Set(v string) error {
	*r = append(*r, v)
	return nil
}

type validateOpts struct {
	src       string
	out       string
	baselines repeatedFlag
	srcLang   string
	dstLang   string
	jsonOut   string
	limit     int
}

func registerValidateFlags(fs *flag.FlagSet, o *validateOpts) {
	fs.StringVar(&o.src, "src", "", "path to the IV2 mod directory")
	fs.StringVar(&o.out, "out", "", "path to the generated Korean pack (omit to check the source only)")
	fs.Var(&o.baselines, "baseline", "localization tree we must not redefine (base game or another mod); repeatable")
	fs.StringVar(&o.srcLang, "src-lang", "english", "source language")
	fs.StringVar(&o.dstLang, "dst-lang", "korean", "target language")
	fs.StringVar(&o.jsonOut, "json", "", "write the full report to this JSON file")
	fs.IntVar(&o.limit, "limit", 20, "how many findings to print per rule")
}

func runValidate(args []string) error {
	var o validateOpts
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	registerValidateFlags(fs, &o)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if o.src == "" {
		return fmt.Errorf("--src is required")
	}

	res, err := validate.Run(validate.Options{
		Source:     o.src,
		Out:        o.out,
		Baselines:  o.baselines,
		SourceLang: o.srcLang,
		TargetLang: o.dstLang,
	})
	if err != nil {
		return err
	}

	reportValidate(o, res)

	if o.jsonOut != "" {
		f, err := os.Create(o.jsonOut)
		if err != nil {
			return err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		payload := struct {
			*validate.Report
			Shadowed       []string `json:"shadowed_keys,omitempty"`
			DoNotTranslate []string `json:"do_not_translate_keys,omitempty"`
		}{res.Report, res.Shadowed, res.DoNotTranslateKeys}
		if err := enc.Encode(payload); err != nil {
			return err
		}
		fmt.Printf("\nreport written to %s\n", o.jsonOut)
	}

	if res.Report.Errors() > 0 {
		return fmt.Errorf("%d blocking finding(s)", res.Report.Errors())
	}
	return nil
}

func reportValidate(o validateOpts, res *validate.Result) {
	p := func(format string, a ...any) { fmt.Printf(format, a...) }
	rep := res.Report

	p("═══ validate ═══\n")
	p("  source   %s  (%d keys in %d files)\n", o.src, len(res.Source.Entries), len(res.Source.Files))
	if res.Out != nil {
		p("  output   %s  (%d keys in %d files)\n", o.out, len(res.Out.Entries), len(res.Out.Files))
	} else {
		p("  output   (none given; source-side checks only)\n")
	}

	p("\n═══ base-game / foreign-mod collisions ═══\n")
	if !rep.BaselineChecked {
		// Saying nothing here would read as a pass. The guarantee is
		// unproven until a baseline is supplied, and it is the guarantee
		// whose absence crashed the previous pack.
		p("  NOT CHECKED - no --baseline given.\n")
		p("  Pass the base game's root and every other mod in the load order to\n")
		p("  prove the pack redefines nothing. Point --baseline at the directory\n")
		p("  above the layers, not at a localization folder; EU5 splits its own\n")
		p("  localization across dlc/, in_game/, loading_screen/ and main_menu/,\n")
		p("  and the scan walks all of them:\n")
		p("    --baseline <EU5>/game --baseline <FUM> --baseline <GlorpUI> --baseline <CMM>\n")
	} else {
		p("  baseline keys known              %8d\n", len(res.BaselineKeys))
		p("  IV2 keys that shadow a baseline  %8d\n", len(res.Shadowed))

		byLayer := map[string]int{}
		for _, k := range res.Shadowed {
			byLayer[res.BaselineKeys[k].Layer]++
		}
		if len(byLayer) > 0 {
			layers := make([]string, 0, len(byLayer))
			for l := range byLayer {
				layers = append(layers, l)
			}
			sort.Strings(layers)
			p("\n  by baseline layer:\n")
			for _, l := range layers {
				p("    %-20s %6d\n", l, byLayer[l])
			}
		}
		if len(res.Shadowed) > 0 {
			p("\n  IV2 redefines these keys itself. Our pack must leave them alone:\n")
			for i, k := range res.Shadowed {
				if i >= o.limit {
					p("    ... %d more\n", len(res.Shadowed)-o.limit)
					break
				}
				p("    %-56s %s\n", trunc(k, 56), res.BaselineKeys[k].Where())
			}
		}
	}

	p("\n═══ do-not-translate keys ═══\n")
	p("  %d source keys hold engine tokens rather than display text\n", len(res.DoNotTranslateKeys))
	for i, k := range res.DoNotTranslateKeys {
		if i >= o.limit {
			p("    ... %d more\n", len(res.DoNotTranslateKeys)-o.limit)
			break
		}
		p("    %s\n", k)
	}

	p("\n═══ findings ═══\n")
	if len(rep.Findings) == 0 {
		p("  none\n")
	} else {
		p("  %-22s %-8s %6s\n", "RULE", "SEVERITY", "COUNT")
		for _, rc := range rep.ByRule() {
			p("  %-22s %-8s %6d\n", rc.Rule, rc.Severity, rc.Count)
		}

		shown := map[string]int{}
		p("\n")
		for _, f := range rep.Findings {
			if shown[f.Rule] >= o.limit {
				continue
			}
			shown[f.Rule]++
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			p("  [%s] %s %s\n", f.Severity, f.Rule, f.Key)
			if loc != "" {
				p("      at %s\n", loc)
			}
			p("      %s\n", f.Message)
			if f.Source != "" {
				p("      en: %s\n", trunc(oneLine(f.Source), 100))
			}
			if f.Translation != "" {
				p("      ko: %s\n", trunc(oneLine(f.Translation), 100))
			}
		}
		for _, rc := range rep.ByRule() {
			if rc.Count > shown[rc.Rule] {
				p("  ... %d more %s finding(s) (--limit to show more)\n",
					rc.Count-shown[rc.Rule], rc.Rule)
			}
		}
	}

	p("\n═══ result ═══\n")
	p("  errors   %d\n", rep.Errors())
	p("  warnings %d\n", rep.Warnings())
	if n := len(rep.FallbackKeys); n > 0 {
		p("  %d key(s) must ship as English; build will fall back automatically\n", n)
	}
	if rep.Errors() == 0 && rep.BaselineChecked {
		p("\n  PASS\n")
	} else if rep.Errors() == 0 {
		p("\n  PASS (collision check not proven; see above)\n")
	} else {
		p("\n  FAIL\n")
	}
}
