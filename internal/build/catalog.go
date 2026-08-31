// Package build renders a Korean localization pack from IV2's source and a
// catalog of translations.
package build

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/inventory"
)

// CatalogVersion is the on-disk format version.
const CatalogVersion = 1

// Unit is one translation unit: an English template and its Korean
// counterpart. Keying by the English template rather than by key means that
// when IV2 changes a string, its template changes, no unit matches, and the
// string is retranslated automatically. That is what keeps the pack from
// rotting the way the previous one did.
type Unit struct {
	Template string `json:"template"`
	// Korean is the translated template, empty while untranslated.
	Korean string `json:"korean"`
	// Rep and Members are context for a human reviewing the catalog: which
	// key this text came from and how many keys it covers.
	Rep     string `json:"rep,omitempty"`
	Class   string `json:"class,omitempty"`
	Members int    `json:"members,omitempty"`
}

// Catalog is the translation store. It is a list rather than a map so that it
// diffs readably in git.
type Catalog struct {
	Version int    `json:"version"`
	Units   []Unit `json:"units"`
}

// NewCatalog builds an untranslated catalog from a set of groups.
func NewCatalog(groups []inventory.Group) *Catalog {
	c := &Catalog{Version: CatalogVersion}
	for _, g := range groups {
		c.Units = append(c.Units, Unit{
			Template: g.Template,
			Rep:      g.Rep,
			Class:    string(g.Class),
			Members:  len(g.Members),
		})
	}
	return c
}

// Index maps English template to Korean template, skipping untranslated units.
func (c *Catalog) Index() map[string]string {
	if c == nil {
		return nil
	}
	m := make(map[string]string, len(c.Units))
	for _, u := range c.Units {
		if u.Korean != "" {
			m[u.Template] = u.Korean
		}
	}
	return m
}

// Translated counts the units carrying a translation.
func (c *Catalog) Translated() int {
	if c == nil {
		return 0
	}
	n := 0
	for _, u := range c.Units {
		if u.Korean != "" {
			n++
		}
	}
	return n
}

// Sort orders units by class then representative key, so that regenerating a
// catalog produces a stable file.
func (c *Catalog) Sort() {
	sort.SliceStable(c.Units, func(i, j int) bool {
		if c.Units[i].Class != c.Units[j].Class {
			return c.Units[i].Class < c.Units[j].Class
		}
		return c.Units[i].Rep < c.Units[j].Rep
	})
}

// ReadCatalog loads a catalog from disk.
func ReadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if c.Version != CatalogVersion {
		return nil, fmt.Errorf("%s: catalog version %d, want %d", path, c.Version, CatalogVersion)
	}
	return &c, nil
}

// WriteCatalog saves a catalog.
func WriteCatalog(path string, c *Catalog) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(c)
}
