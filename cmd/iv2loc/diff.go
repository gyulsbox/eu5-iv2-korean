package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/gyulsbox/eu5-iv2-korean/internal/build"
	"github.com/gyulsbox/eu5-iv2-korean/internal/diff"
	"github.com/gyulsbox/eu5-iv2-korean/internal/inventory"
)

type diffOpts struct {
	src     string
	catalog string
	out     string
	lang    string
	dstLang string
	update  bool
	prune   bool
	jsonOut string
	limit   int
}

func registerDiffFlags(fs *flag.FlagSet, o *diffOpts) {
	fs.StringVar(&o.src, "src", "", "path to the IV2 mod directory")
	fs.StringVar(&o.catalog, "catalog", "", "catalog to compare against")
	fs.StringVar(&o.out, "out", "", "previously built pack, for the key-level view")
	fs.StringVar(&o.lang, "lang", "english", "source language")
	fs.StringVar(&o.dstLang, "dst-lang", "korean", "target language")
	fs.BoolVar(&o.update, "update", false, "write the merged catalog back, keeping every translation whose English is unchanged")
	fs.BoolVar(&o.prune, "prune", false, "with --update, drop units whose text IV2 no longer has")
	fs.StringVar(&o.jsonOut, "json", "", "write the full report to this JSON file")
	fs.IntVar(&o.limit, "limit", 10, "entries to print per section")
}

func runDiff(args []string) error {
	var o diffOpts
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	registerDiffFlags(fs, &o)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if o.src == "" {
		return fmt.Errorf("--src is required")
	}
	if o.catalog == "" {
		return fmt.Errorf("--catalog is required")
	}

	inv, err := inventory.Scan(o.src, o.lang)
	if err != nil {
		return err
	}
	if len(inv.Entries) == 0 {
		return fmt.Errorf("no *_l_%s.yml keys found under %s", o.lang, o.src)
	}
	groups := inventory.Groups(inv.Entries)

	cat, err := build.ReadCatalog(o.catalog)
	if err != nil {
		return err
	}

	var pack *inventory.Inventory
	if o.out != "" {
		pack, err = inventory.Scan(o.out, o.dstLang)
		if err != nil {
			return err
		}
	}

	rep := diff.Compare(groups, inv.Entries, cat, pack)
	reportDiff(o, rep, cat, len(groups))

	if o.jsonOut != "" {
		f, err := os.Create(o.jsonOut)
		if err != nil {
			return err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(rep); err != nil {
			return err
		}
		fmt.Printf("\nreport written to %s\n", o.jsonOut)
	}

	if o.update {
		st := diff.Merge(cat, groups, o.prune)
		if err := build.WriteCatalog(o.catalog, cat); err != nil {
			return err
		}
		fmt.Printf("\n═══ catalog updated ═══\n")
		fmt.Printf("  kept       %8d   (translations preserved)\n", st.Kept)
		fmt.Printf("  added      %8d   (new, awaiting translation)\n", st.Added)
		if st.Refresh > 0 {
			fmt.Printf("  refreshed  %8d   (same text, different key coverage)\n", st.Refresh)
		}
		if o.prune {
			fmt.Printf("  pruned     %8d\n", st.Pruned)
		}
		fmt.Printf("  catalog is now %d units, %d translated\n", len(cat.Units), cat.Translated())
		if n := cat.Translated(); n < len(cat.Units) {
			fmt.Printf("\n  next: iv2loc translate --catalog %s --glossary glossary.json\n", o.catalog)
		}
	} else if rep.Work() > 0 {
		fmt.Printf("\n  to apply: rerun with --update\n")
	}
	return nil
}

func reportDiff(o diffOpts, rep *diff.Report, cat *build.Catalog, groups int) {
	p := func(format string, a ...any) { fmt.Printf(format, a...) }

	p("═══ diff ═══\n")
	p("  source    %s  (%d translation units)\n", o.src, groups)
	p("  catalog   %s  (%d units, %d translated)\n", o.catalog, len(cat.Units), cat.Translated())
	if rep.PackCompared {
		p("  pack      %s\n", o.out)
	}

	p("\n═══ units ═══\n")
	p("  unchanged        %8d\n", rep.Unchanged)
	p("  changed          %8d   (IV2 reworded the text)\n", len(rep.Changed))
	p("  new              %8d\n", len(rep.New))
	p("  orphaned         %8d   (IV2 no longer has this text)\n", len(rep.Orphaned))
	p("  still untranslated %6d\n", rep.Untranslated)
	p("\n  >> %d unit(s) need translating\n", rep.Work())

	section := func(title string, cs []diff.UnitChange, showPrev bool) {
		if len(cs) == 0 {
			return
		}
		p("\n═══ %s ═══\n", title)
		for i, c := range cs {
			if i >= o.limit {
				p("  ... %d more\n", len(cs)-o.limit)
				break
			}
			p("  x%-5d %-44s %s\n", c.Members, trunc(c.Rep, 44), trunc(oneLine(c.Template), 70))
			if showPrev {
				p("         was: %s\n", trunc(oneLine(c.PreviousTemplate), 70))
				if c.PreviousKorean != "" {
					p("         old ko: %s\n", trunc(oneLine(c.PreviousKorean), 70))
				}
			}
		}
	}
	section("changed", rep.Changed, true)
	section("new", rep.New, false)
	section("orphaned", rep.Orphaned, false)

	if rep.PackCompared {
		p("\n═══ keys vs the built pack ═══\n")
		p("  added            %8d\n", len(rep.AddedKeys))
		p("  removed          %8d\n", len(rep.RemovedKeys))
		for _, set := range []struct {
			name string
			keys []string
		}{{"added", rep.AddedKeys}, {"removed", rep.RemovedKeys}} {
			if len(set.keys) == 0 {
				continue
			}
			p("\n  %s:\n", set.name)
			for i, k := range set.keys {
				if i >= o.limit {
					p("    ... %d more\n", len(set.keys)-o.limit)
					break
				}
				p("    %s\n", k)
			}
		}
	} else {
		p("\n  (pass --out <pack> for the key-level view)\n")
	}
}
