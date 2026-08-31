package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/gyulsbox/eu5-iv2-korean/internal/inventory"
	"github.com/gyulsbox/eu5-iv2-korean/internal/validate"
)

type keysOpts struct {
	srcs  repeatedFlag
	langs repeatedFlag
	out   string
}

func registerKeysFlags(fs *flag.FlagSet, o *keysOpts) {
	fs.Var(&o.srcs, "src", "localization tree to read; repeatable")
	fs.Var(&o.langs, "lang", "language to read; repeatable (default english and korean)")
	fs.StringVar(&o.out, "out", "", "write the digest here instead of stdout")
}

// runKeys writes a key digest: the layer and name of every localization key in
// a tree, and nothing else.
//
// It exists because the collision check needs to know which keys the base game
// owns, and nothing more. The values are what make a localization tree large,
// so dropping them turns a share that is impractical into a small text file.
func runKeys(args []string) error {
	var o keysOpts
	fs := flag.NewFlagSet("keys", flag.ContinueOnError)
	registerKeysFlags(fs, &o)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(o.srcs) == 0 {
		return fmt.Errorf("--src is required")
	}
	langs := o.langs
	if len(langs) == 0 {
		langs = []string{"english", "korean"}
	}

	// Layer is kept because collisions are reported per layer, and a key can
	// legitimately appear in more than one.
	seen := map[validate.KeyRef]struct{}{}
	var files int
	for _, src := range o.srcs {
		for _, lang := range langs {
			inv, err := inventory.Scan(src, lang)
			if err != nil {
				return fmt.Errorf("scanning %s: %w", src, err)
			}
			files += len(inv.Files)
			for _, e := range inv.Entries {
				seen[validate.KeyRef{Layer: e.Layer, Key: e.Key}] = struct{}{}
			}
		}
	}

	refs := make([]validate.KeyRef, 0, len(seen))
	for r := range seen {
		refs = append(refs, r)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Key != refs[j].Key {
			return refs[i].Key < refs[j].Key
		}
		return refs[i].Layer < refs[j].Layer
	})

	out := os.Stdout
	if o.out != "" {
		f, err := os.Create(o.out)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	w := bufio.NewWriter(out)
	fmt.Fprintf(w, "%s\n", validate.KeyDigestHeader)
	for _, src := range o.srcs {
		fmt.Fprintf(w, "# source: %s\n", src)
	}
	for _, r := range refs {
		fmt.Fprintf(w, "%s\t%s\n", r.Layer, r.Key)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	if o.out != "" {
		fi, err := os.Stat(o.out)
		if err == nil {
			fmt.Fprintf(os.Stderr, "%d keys from %d files -> %s (%.1f KB)\n",
				len(refs), files, o.out, float64(fi.Size())/1024)
		}
	}
	return nil
}
