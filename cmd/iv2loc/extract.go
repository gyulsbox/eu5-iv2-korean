package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gyulsbox/eu5-iv2-korean/internal/inventory"
	"github.com/gyulsbox/eu5-iv2-korean/internal/paradox"
)

type extractOpts struct {
	src      string
	lang     string
	jsonOut  string
	topN     int
	showRefs bool
}

func registerExtractFlags(fs *flag.FlagSet, o *extractOpts) {
	fs.StringVar(&o.src, "src", "", "path to the IV2 mod directory")
	fs.StringVar(&o.lang, "lang", "english", "source language to extract")
	fs.StringVar(&o.jsonOut, "json", "", "write the full inventory to this JSON file")
	fs.IntVar(&o.topN, "top", 15, "how many largest translation groups to list")
	fs.BoolVar(&o.showRefs, "show-foreign", false, "list every key lacking the mod namespace")
}

func runExtract(args []string) error {
	var o extractOpts
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	registerExtractFlags(fs, &o)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if o.src == "" {
		return fmt.Errorf("--src is required")
	}

	inv, err := inventory.Scan(o.src, o.lang)
	if err != nil {
		return err
	}
	if len(inv.Files) == 0 {
		return fmt.Errorf("no *_l_%s.yml files found under %s", o.lang, o.src)
	}

	groups := inventory.Groups(inv.Entries)
	stats := inventory.Summarize(inv.Entries, groups)

	report(os.Stdout, o, inv, groups, stats)

	if o.jsonOut != "" {
		payload := struct {
			Stats     inventory.Stats     `json:"stats"`
			Files     []inventory.File    `json:"files"`
			Groups    []inventory.Group   `json:"groups"`
			Entries   []inventory.Entry   `json:"entries"`
			Duplicate map[string][]string `json:"duplicates,omitempty"`
		}{stats, inv.Files, groups, inv.Entries, inv.Duplicates}

		f, err := os.Create(o.jsonOut)
		if err != nil {
			return err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return err
		}
		fmt.Printf("\ninventory written to %s\n", o.jsonOut)
	}
	return nil
}

func report(w *os.File, o extractOpts, inv *inventory.Inventory, groups []inventory.Group, s inventory.Stats) {
	p := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }

	p("═══ source ═══\n")
	p("%s  (language: %s)\n\n", inv.Source, o.lang)

	p("%-46s %8s %6s %s\n", "FILE", "KEYS", "BOM", "LAYER")
	var noBOM, parseErrs int
	for _, f := range inv.Files {
		bom := "yes"
		if !f.HadBOM {
			bom = "NO"
			noBOM++
		}
		p("%-46s %8d %6s %s\n", trunc(f.Path, 46), f.Keys, bom, f.Layer)
		for _, e := range f.Errors {
			parseErrs++
			p("    ! %s\n", e)
		}
	}

	p("\n═══ key inventory ═══\n")
	p("  total keys defined            %8d\n", s.TotalKeys)
	p("  unique values (exact)         %8d\n", s.UniqueValues)
	p("  translation groups            %8d   (markup + numbers lifted into placeholders)\n", s.Groups)
	if s.MalformedKeys > 0 {
		p("  malformed in source           %8d   (recovered, see parse errors above)\n", s.MalformedKeys)
	}

	p("\n═══ what actually needs translating ═══\n")
	p("  %-22s %8s %10s\n", "CLASS", "KEYS", "GROUPS")
	p("  %-22s %8d %10d\n", "empty", s.EmptyKeys, s.EmptyGroups)
	p("  %-22s %8d %10d\n", "reference-only", s.ReferenceKeys, s.ReferenceGroups)
	p("  %-22s %8d %10d\n", "prose", s.ProseKeys, s.ProseGroups)
	p("\n  >> %d unique strings need a translation call\n", s.ProseGroups)
	p("  >> %d characters of source prose (%.1f KB)\n", s.ProseChars, float64(s.ProseChars)/1024)
	if s.TotalKeys > 0 && s.ProseGroups > 0 {
		p("  >> reduction from %d keys: %.1fx\n", s.TotalKeys, float64(s.TotalKeys)/float64(s.ProseGroups))
	}

	p("\n═══ token usage ═══\n")
	kinds := map[paradox.TokenKind]int{}
	distinct := map[string]struct{}{}
	for _, e := range inv.Entries {
		for _, t := range paradox.Tokens(e.Value) {
			kinds[t.Kind]++
			distinct[t.Text] = struct{}{}
		}
	}
	order := []paradox.TokenKind{
		paradox.KindVariable, paradox.KindScope, paradox.KindFormat,
		paradox.KindFormatOff, paradox.KindIcon, paradox.KindTextIcon,
		paradox.KindNewline,
	}
	for _, k := range order {
		p("  %-12s %8d\n", k, kinds[k])
	}
	p("  %-12s %8d\n", "distinct", len(distinct))

	p("\n═══ namespace audit ═══\n")
	foreign := inventory.Foreign(inv.Entries)
	p("  keys carrying the IV2 namespace   %8d\n", s.TotalKeys-len(foreign))
	p("  keys WITHOUT it (review these)    %8d\n", len(foreign))
	if len(foreign) > 0 {
		shapes := inventory.ForeignShapes(inv.Entries)
		p("  ...which reduce to %d key shapes:\n", len(shapes))
		limit := len(shapes)
		if !o.showRefs && limit > 25 {
			limit = 25
		}
		for _, sh := range shapes[:limit] {
			p("    %6d  %-56s e.g. %s\n", sh.Count, trunc(sh.Shape, 56), trunc(sh.Example, 40))
		}
		if limit < len(shapes) {
			p("    ... %d more shapes (--show-foreign to list all)\n", len(shapes)-limit)
		}
	}

	p("\n  top key prefixes:\n")
	for i, pc := range inventory.Prefixes(inv.Entries, 3) {
		if i >= 10 {
			break
		}
		p("    %-40s %6d\n", pc.Prefix, pc.Count)
	}

	if len(inv.Duplicates) > 0 {
		p("\n═══ duplicate keys ═══\n")
		keys := make([]string, 0, len(inv.Duplicates))
		for k := range inv.Duplicates {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i >= 20 {
				p("  ... %d more\n", len(keys)-20)
				break
			}
			p("  %-46s %s\n", trunc(k, 46), strings.Join(inv.Duplicates[k], ", "))
		}
	}

	p("\n═══ largest prose groups (highest leverage) ═══\n")
	shown := 0
	for _, g := range groups {
		if g.Class != inventory.ClassProse || shown >= o.topN {
			continue
		}
		shown++
		p("  x%-5d %s\n", len(g.Members), trunc(oneLine(g.Template), 92))
	}

	if noBOM > 0 {
		p("\n! %d source file(s) lack a UTF-8 BOM\n", noBOM)
	}
	if parseErrs > 0 {
		p("\n! %d line(s) failed to parse\n", parseErrs)
	}
}

func oneLine(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\t", " ")
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
