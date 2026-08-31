package build

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gyulsbox/eu5-iv2-korean/internal/inventory"
	"github.com/gyulsbox/eu5-iv2-korean/internal/korean"
	"github.com/gyulsbox/eu5-iv2-korean/internal/paradox"
	"github.com/gyulsbox/eu5-iv2-korean/internal/validate"
)

// LoadOrderPrefix is prepended to every generated file name.
//
// EU5 processes localization files in reverse alphabetical order and a key
// defined earlier wins, so a leading `0` puts our pack last and makes it
// deferential: if anything else already defines one of these keys in Korean,
// theirs stands. We are adding a language layer IV2 lacks, not competing for
// keys, so losing a race is the outcome we want.
const LoadOrderPrefix = "0_"

// Options configures a build.
type Options struct {
	// Source is the IV2 mod directory.
	Source string
	// Out is the mod directory to generate.
	Out string
	// Catalog holds the translations; nil builds an English passthrough,
	// which is how the pipeline is verified before any translation exists.
	Catalog *Catalog

	SourceLang string
	TargetLang string

	// Meta describes the generated mod.
	Meta Metadata
	// SingleFile writes one combined file instead of mirroring IV2's layout.
	SingleFile bool
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

// Stats summarizes what a build produced.
type Stats struct {
	Files        int      `json:"files"`
	Keys         int      `json:"keys"`
	Translated   int      `json:"translated"`
	English      int      `json:"english"`
	DoNotTouch   int      `json:"do_not_translate"`
	FellBack     int      `json:"fell_back"`
	FallbackKeys []string `json:"fallback_keys,omitempty"`
	Bytes        int64    `json:"bytes"`
}

// Run generates the pack.
func Run(o Options) (*Stats, error) {
	src, err := inventory.Scan(o.Source, o.sourceLang())
	if err != nil {
		return nil, fmt.Errorf("scanning source: %w", err)
	}
	if len(src.Entries) == 0 {
		return nil, fmt.Errorf("no *_l_%s.yml keys found under %s", o.sourceLang(), o.Source)
	}

	groups := inventory.Groups(src.Entries)
	translations := o.Catalog.Index()

	// Map each key to the group covering it, along with the substitutions
	// that rebuild that key's own string from the group template.
	type binding struct {
		template string
		member   inventory.Member
	}
	bind := make(map[string]binding, len(src.Entries))
	for _, g := range groups {
		for _, m := range g.Members {
			bind[m.Key] = binding{template: g.Template, member: m}
		}
	}

	stats := &Stats{}
	// Entries keep source order so the output diffs against IV2 readably.
	byFile := map[string][]paradox.Entry{}
	var fileOrder []string

	for _, e := range src.Entries {
		value := e.Value

		switch {
		case validate.DoNotTranslate(e.Key, e.Value):
			// Copied verbatim: the engine reads this value, it is not shown.
			stats.DoNotTouch++
		default:
			b := bind[e.Key]
			if ko, ok := translations[b.template]; ok {
				if rendered, filled := inventory.Detemplatize(ko, b.member.Tokens, b.member.Numbers); filled {
					// Resolve particles once more now that the placeholders
					// are gone. A ⟦N1⟧ has become a literal number, so the
					// particle after it is no longer a guess: the template's
					// "⟦N1⟧으로(로)" becomes "4로". A ⟦T1⟧ has become engine
					// markup the game expands at runtime, so the particle
					// after that one stays paired.
					value = korean.Polish(rendered)
				}
			}
		}

		// The last gate before anything reaches disk. A translation that
		// damaged its markup is replaced by English here, which is what makes
		// a bad translation unable to break the game.
		safe := validate.Fallback(e.Key, e.Value, value)
		if safe != value {
			stats.FellBack++
			stats.FallbackKeys = append(stats.FallbackKeys, e.Key)
			value = safe
		}

		if value == e.Value {
			stats.English++
		} else {
			stats.Translated++
		}

		out := outputPath(o, e)
		if _, seen := byFile[out]; !seen {
			fileOrder = append(fileOrder, out)
		}
		byFile[out] = append(byFile[out], paradox.Entry{
			Key: e.Key, Version: e.Version, Value: value,
		})
		stats.Keys++
	}
	sort.Strings(stats.FallbackKeys)

	for _, rel := range fileOrder {
		n, err := writeLocFile(filepath.Join(o.Out, rel), o.targetLang(), byFile[rel])
		if err != nil {
			return nil, err
		}
		stats.Files++
		stats.Bytes += n
	}

	if err := writeMetadata(o); err != nil {
		return nil, err
	}
	return stats, nil
}

// outputPath decides where a source key's translation belongs. It mirrors
// IV2's own layer, because EU5 loads dlc, in_game, loading_screen and
// main_menu separately and a file under the wrong one is never read.
func outputPath(o Options, e inventory.Entry) string {
	dir := filepath.Join(e.Layer, "localization", o.targetLang())
	if o.SingleFile {
		return filepath.Join(dir, LoadOrderPrefix+"iv2_korean_l_"+o.targetLang()+".yml")
	}
	base := filepath.Base(e.File)
	base = strings.TrimSuffix(base, "_l_"+o.sourceLang()+".yml")
	return filepath.Join(dir, LoadOrderPrefix+base+"_l_"+o.targetLang()+".yml")
}

// writeLocFile writes one localization file, BOM first.
func writeLocFile(path, lang string, entries []paradox.Entry) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	// Without the BOM, EU5 ignores the file and reports nothing at all.
	if _, err := w.WriteString(paradox.BOM); err != nil {
		return 0, err
	}
	// The header names the language with its `l_` prefix; `korean:` alone
	// parses as nothing and the game reads no keys from the file.
	fmt.Fprintf(w, "l_%s:\n", lang)
	for _, e := range entries {
		if e.Version >= 0 {
			fmt.Fprintf(w, " %s:%d %s\n", e.Key, e.Version, paradox.Quote(e.Value))
		} else {
			fmt.Fprintf(w, " %s: %s\n", e.Key, paradox.Quote(e.Value))
		}
	}
	if err := w.Flush(); err != nil {
		return 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// Metadata describes the generated mod to the launcher.
type Metadata struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	// Version is our pack's version, not the game's.
	Version string `json:"version"`
	// SupportedGameVersion must track the game. The previous Korean pack
	// declared 1.2 against a 1.3.11 game and the launcher flagged it as the
	// only mismatched mod in the load order.
	SupportedGameVersion string         `json:"supported_game_version"`
	ShortDescription     string         `json:"short_description"`
	Tags                 []string       `json:"tags"`
	Relationships        []Relationship `json:"relationships,omitempty"`
	Picture              string         `json:"picture"`
	GameCustomData       map[string]any `json:"game_custom_data"`
}

// Relationship declares a dependency on another mod.
type Relationship struct {
	RelType      string `json:"rel_type"`
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	ResourceType string `json:"resource_type"`
	Version      string `json:"version"`
}

// DefaultMetadata returns the metadata for the IV2 Korean pack.
func DefaultMetadata() Metadata {
	return Metadata{
		Name:                 "The Idea Variation 2 - Korean",
		ID:                   "iv2_korean",
		Version:              "0.1.0",
		SupportedGameVersion: "1.3.*",
		ShortDescription:     "Korean localization for The Idea Variation 2.",
		Tags:                 []string{"Localization"},
		Relationships: []Relationship{{
			RelType:      "dependency",
			ID:           "Idea_Variation_2",
			DisplayName:  "The Idea Variation 2",
			ResourceType: "mod",
			Version:      "2.*",
		}},
		Picture:        "thumbnail.png",
		GameCustomData: map[string]any{},
	}
}

func writeMetadata(o Options) error {
	dir := filepath.Join(o.Out, ".metadata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(o.Meta, "", "\t")
	if err != nil {
		return err
	}
	// IV2's own metadata.json carries a BOM; match it.
	body := append([]byte(paradox.BOM), data...)
	body = append(body, '\n')
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), body, 0o644); err != nil {
		return err
	}

	// A thumbnail is required, but never overwrite one the user supplied.
	thumb := filepath.Join(dir, o.Meta.Picture)
	if _, err := os.Stat(thumb); os.IsNotExist(err) {
		return writePlaceholderThumbnail(thumb)
	}
	return nil
}
