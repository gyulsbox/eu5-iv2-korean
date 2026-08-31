package main

import (
	"flag"
	"fmt"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/build"
	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/translate"
)

type polishOpts struct {
	catalog string
	limit   int
}

func registerPolishFlags(fs *flag.FlagSet, o *polishOpts) {
	fs.StringVar(&o.catalog, "catalog", "", "catalog to clean up, written back in place")
	fs.IntVar(&o.limit, "limit", 10, "how many changes to print")
}

// runPolish applies the mechanical Korean cleanups to an existing catalog.
// `translate` already does this to everything it writes; this exists for a
// catalog filled in some other way, or after the rules here change.
func runPolish(args []string) error {
	var o polishOpts
	fs := flag.NewFlagSet("polish", flag.ContinueOnError)
	registerPolishFlags(fs, &o)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if o.catalog == "" {
		return fmt.Errorf("--catalog is required")
	}

	cat, err := build.ReadCatalog(o.catalog)
	if err != nil {
		return err
	}

	var changed int
	var samples [][2]string
	for i, u := range cat.Units {
		if u.Korean == "" {
			continue
		}
		fixed := translate.PolishCatalogValue(u.Korean)
		if fixed == u.Korean {
			continue
		}
		cat.Units[i].Korean = fixed
		changed++
		if len(samples) < o.limit {
			samples = append(samples, [2]string{u.Korean, fixed})
		}
	}

	if err := build.WriteCatalog(o.catalog, cat); err != nil {
		return err
	}

	fmt.Printf("═══ polish ═══\n")
	fmt.Printf("  catalog   %s\n", o.catalog)
	fmt.Printf("  changed   %d of %d translated units\n", changed, cat.Translated())
	for _, s := range samples {
		fmt.Printf("\n    - %s\n    + %s\n", trunc(oneLine(s[0]), 100), trunc(oneLine(s[1]), 100))
	}
	if changed > len(samples) {
		fmt.Printf("\n  ... %d more\n", changed-len(samples))
	}
	return nil
}
