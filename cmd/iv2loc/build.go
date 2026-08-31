package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gyulsbox/eu5-iv2-korean/internal/build"
	"github.com/gyulsbox/eu5-iv2-korean/internal/inventory"
	"github.com/gyulsbox/eu5-iv2-korean/internal/validate"
)

type buildOpts struct {
	src        string
	out        string
	catalog    string
	initCat    string
	name       string
	id         string
	version    string
	gameVer    string
	singleFile bool
	skipCheck  bool
	limit      int
}

func registerBuildFlags(fs *flag.FlagSet, o *buildOpts) {
	fs.StringVar(&o.src, "src", "", "path to the IV2 mod directory")
	fs.StringVar(&o.out, "out", "", "mod directory to generate")
	fs.StringVar(&o.catalog, "catalog", "", "translation catalog; omit to build an English passthrough")
	fs.StringVar(&o.initCat, "init-catalog", "", "write an empty catalog for every translation unit here and stop")
	fs.StringVar(&o.name, "name", "", "mod display name")
	fs.StringVar(&o.id, "id", "", "mod id (ASCII)")
	fs.StringVar(&o.version, "version", "", "pack version")
	fs.StringVar(&o.gameVer, "game-version", "", "supported game version, e.g. 1.3.*")
	fs.BoolVar(&o.singleFile, "single-file", false, "write one combined file instead of mirroring IV2's layout")
	fs.BoolVar(&o.skipCheck, "no-validate", false, "skip the validation pass that normally follows a build")
	fs.IntVar(&o.limit, "limit", 10, "findings to print per rule in the validation pass")
}

func runBuild(args []string) error {
	var o buildOpts
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	registerBuildFlags(fs, &o)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if o.src == "" {
		return fmt.Errorf("--src is required")
	}

	if o.initCat != "" {
		return initCatalog(o)
	}
	if o.out == "" {
		return fmt.Errorf("--out is required")
	}

	meta := build.DefaultMetadata()
	if o.name != "" {
		meta.Name = o.name
	}
	if o.id != "" {
		meta.ID = o.id
	}
	if o.version != "" {
		meta.Version = o.version
	}
	if o.gameVer != "" {
		meta.SupportedGameVersion = o.gameVer
	}
	// A non-ASCII mod id or path is a documented way to make EU5 fail to load
	// a mod, so refuse before writing anything.
	if !validate.ASCIIPath(meta.ID) {
		return fmt.Errorf("mod id %q contains non-ASCII characters; EU5 will not load it", meta.ID)
	}
	if !validate.ASCIIPath(o.out) {
		return fmt.Errorf("output path %q contains non-ASCII characters; EU5 will not load it", o.out)
	}

	var cat *build.Catalog
	if o.catalog != "" {
		c, err := build.ReadCatalog(o.catalog)
		if err != nil {
			return err
		}
		cat = c
	}

	stats, err := build.Run(build.Options{
		Source:     o.src,
		Out:        o.out,
		Catalog:    cat,
		Meta:       meta,
		SingleFile: o.singleFile,
	})
	if err != nil {
		return err
	}

	fmt.Printf("═══ build ═══\n")
	fmt.Printf("  out              %s\n", o.out)
	if cat == nil {
		fmt.Printf("  catalog          (none - English passthrough)\n")
	} else {
		fmt.Printf("  catalog          %s (%d of %d units translated)\n",
			o.catalog, cat.Translated(), len(cat.Units))
	}
	fmt.Printf("  files written    %8d\n", stats.Files)
	fmt.Printf("  keys written     %8d\n", stats.Keys)
	fmt.Printf("  translated       %8d\n", stats.Translated)
	fmt.Printf("  left in English  %8d\n", stats.English)
	fmt.Printf("  copied verbatim  %8d   (engine tokens)\n", stats.DoNotTouch)
	fmt.Printf("  total size       %8.1f KB\n", float64(stats.Bytes)/1024)
	if stats.FellBack > 0 {
		fmt.Printf("\n  %d translation(s) failed validation and shipped as English:\n", stats.FellBack)
		for i, k := range stats.FallbackKeys {
			if i >= o.limit {
				fmt.Printf("    ... %d more\n", len(stats.FallbackKeys)-o.limit)
				break
			}
			fmt.Printf("    %s\n", k)
		}
	}

	if o.skipCheck {
		return nil
	}

	fmt.Printf("\n")
	return runValidate([]string{
		"--src", o.src, "--out", o.out, "--limit", fmt.Sprint(o.limit),
	})
}

// initCatalog writes a catalog stub covering every translation unit, which is
// the input the translate stage will fill in.
func initCatalog(o buildOpts) error {
	inv, err := inventory.Scan(o.src, "english")
	if err != nil {
		return err
	}
	groups := inventory.Groups(inv.Entries)
	cat := build.NewCatalog(groups)
	cat.Sort()
	if err := build.WriteCatalog(o.initCat, cat); err != nil {
		return err
	}
	fi, err := os.Stat(o.initCat)
	if err != nil {
		return err
	}
	var prose int
	for _, u := range cat.Units {
		if u.Class == string(inventory.ClassProse) {
			prose++
		}
	}
	fmt.Printf("wrote %d units (%d prose) to %s (%.1f KB)\n",
		len(cat.Units), prose, o.initCat, float64(fi.Size())/1024)
	return nil
}
