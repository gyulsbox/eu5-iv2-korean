// Package inventory walks a mod tree, parses its source-language localization
// files, and groups the resulting strings into the smallest set of units that
// actually has to be translated.
package inventory

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/paradox"
)

// Entry is one localization key together with where it came from.
type Entry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	// File is the mod-relative path of the source file.
	File string `json:"file"`
	// Layer is the top-level mod directory the file lives under
	// (in_game, main_menu, loading_screen), which the output must mirror.
	Layer string `json:"layer"`
	Line  int    `json:"line"`
	// Malformed propagates a recovered source defect from the parser.
	Malformed bool `json:"malformed,omitempty"`
	// Replace records that the file sat under a `replace/` directory.
	Replace bool `json:"replace,omitempty"`
}

// Class describes how much work a value needs.
type Class string

const (
	// ClassEmpty is an empty string. Nothing to translate.
	ClassEmpty Class = "empty"
	// ClassReference is a value built purely out of markup — $key$ chains,
	// icons, formatting — with no natural-language text of its own. It costs
	// no translation call; at most it needs word-order review.
	ClassReference Class = "reference"
	// ClassProse is a value containing actual text to translate.
	ClassProse Class = "prose"
)

// Classify decides which class a value falls into by removing every markup
// token and checking whether any letter survives.
func Classify(value string) Class {
	if strings.TrimSpace(value) == "" {
		return ClassEmpty
	}
	stripped := StripTokens(value)
	for _, r := range stripped {
		if unicode.IsLetter(r) {
			return ClassProse
		}
	}
	return ClassReference
}

// StripTokens removes every markup token from a value.
func StripTokens(value string) string {
	toks := paradox.Tokens(value)
	var b strings.Builder
	prev := 0
	for _, t := range toks {
		b.WriteString(value[prev:t.Start])
		prev = t.End
	}
	b.WriteString(value[prev:])
	return b.String()
}

// File is one parsed source file plus the diagnostics gathered from it.
type File struct {
	Path     string   `json:"path"`
	Layer    string   `json:"layer"`
	Language string   `json:"language"`
	HadBOM   bool     `json:"had_bom"`
	Keys     int      `json:"keys"`
	Errors   []string `json:"errors,omitempty"`
}

// Inventory is the full extraction result for one mod.
type Inventory struct {
	Source  string  `json:"source"`
	Files   []File  `json:"files"`
	Entries []Entry `json:"entries"`
	// Duplicates maps a key defined more than once to every place it appears.
	Duplicates map[string][]string `json:"duplicates,omitempty"`
}

// Scan walks root looking for localization files in the given language and
// parses each one. Files are visited in a deterministic order.
func Scan(root, language string) (*Inventory, error) {
	suffix := "_l_" + language + ".yml"
	inv := &Inventory{Source: root}

	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip VCS and metadata noise.
			if name := d.Name(); name == ".git" || name == ".metadata" {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), suffix) {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	seen := map[string]string{} // key -> first location
	for _, p := range paths {
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)

		f, openErr := os.Open(p)
		if openErr != nil {
			return nil, openErr
		}
		parsed, parseErrs := paradox.Parse(f)
		f.Close()

		fi := File{
			Path:     rel,
			Layer:    layerOf(rel),
			Language: parsed.Language,
			HadBOM:   parsed.HadBOM,
			Keys:     len(parsed.Entries),
		}
		for _, e := range parseErrs {
			fi.Errors = append(fi.Errors, e.Error())
		}
		inv.Files = append(inv.Files, fi)

		isReplace := strings.Contains(rel, "/replace/")
		for _, e := range parsed.Entries {
			loc := fmt.Sprintf("%s:%d", rel, e.Line)
			if first, dup := seen[e.Key]; dup {
				if inv.Duplicates == nil {
					inv.Duplicates = map[string][]string{}
				}
				if len(inv.Duplicates[e.Key]) == 0 {
					inv.Duplicates[e.Key] = []string{first}
				}
				inv.Duplicates[e.Key] = append(inv.Duplicates[e.Key], loc)
			} else {
				seen[e.Key] = loc
			}
			inv.Entries = append(inv.Entries, Entry{
				Key:       e.Key,
				Value:     e.Value,
				File:      rel,
				Layer:     fi.Layer,
				Line:      e.Line,
				Replace:   isReplace,
				Malformed: e.Malformed,
			})
		}
	}
	return inv, nil
}

// layerOf returns the top-level mod directory of a relative path, which is the
// EU5 load layer (in_game, main_menu, loading_screen).
func layerOf(rel string) string {
	if i := strings.IndexByte(rel, '/'); i > 0 {
		return rel[:i]
	}
	return "."
}
