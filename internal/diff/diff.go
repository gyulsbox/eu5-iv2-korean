// Package diff compares a mod's current localization against an existing
// catalog and pack, so that an IV2 update can be caught up with rather than
// retranslated from scratch.
//
// The previous Korean pack rotted because there was no way to tell what had
// changed upstream. Everything here exists to make that question cheap.
package diff

import (
	"sort"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/build"
	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/inventory"
)

// Kind classifies a change.
type Kind string

const (
	// KindNew is a template IV2 has that the catalog does not: untranslated
	// text, whether the string is genuinely new or was reworded.
	KindNew Kind = "new"
	// KindOrphaned is a template the catalog has that IV2 no longer does. Its
	// translation is dead weight; it stays unless pruned, since a string can
	// come back.
	KindOrphaned Kind = "orphaned"
	// KindChanged is an orphan and a new template that belong to the same key:
	// IV2 reworded that string. Reported on its own because the old Korean is
	// often a usable starting point, unlike a genuinely new string.
	KindChanged Kind = "changed"
)

// UnitChange is one difference at the translation-unit level.
type UnitChange struct {
	Kind     Kind   `json:"kind"`
	Template string `json:"template"`
	Rep      string `json:"rep"`
	Class    string `json:"class"`
	Members  int    `json:"members"`
	// PreviousTemplate and PreviousKorean are set on KindChanged.
	PreviousTemplate string `json:"previous_template,omitempty"`
	PreviousKorean   string `json:"previous_korean,omitempty"`
}

// Report is the result of a comparison.
type Report struct {
	// Unchanged counts templates present in both, translated or not.
	Unchanged int          `json:"unchanged"`
	New       []UnitChange `json:"new,omitempty"`
	Orphaned  []UnitChange `json:"orphaned,omitempty"`
	Changed   []UnitChange `json:"changed,omitempty"`
	// Untranslated counts units that survive but still have no Korean.
	Untranslated int `json:"untranslated"`

	// AddedKeys and RemovedKeys need a previously built pack to compute; they
	// are the key-level view of the same update.
	AddedKeys   []string `json:"added_keys,omitempty"`
	RemovedKeys []string `json:"removed_keys,omitempty"`
	// PackCompared records whether a pack was available.
	PackCompared bool `json:"pack_compared"`
}

// Work returns how many units need a translator's attention.
func (r *Report) Work() int {
	return len(r.New) + len(r.Changed) + r.Untranslated
}

// Compare diffs the mod's current groups against the catalog, and, when a
// previously built pack is supplied, against its keys too.
func Compare(groups []inventory.Group, entries []inventory.Entry, cat *build.Catalog, pack *inventory.Inventory) *Report {
	rep := &Report{}

	current := make(map[string]inventory.Group, len(groups))
	for _, g := range groups {
		current[g.Template] = g
	}
	// Which template covers a given key now; used to pair an orphan with the
	// new template that replaced it.
	templateOfKey := make(map[string]string, len(entries))
	for _, g := range groups {
		for _, m := range g.Members {
			templateOfKey[m.Key] = g.Template
		}
	}

	inCatalog := make(map[string]build.Unit, len(cat.Units))
	for _, u := range cat.Units {
		inCatalog[u.Template] = u
	}

	newByTemplate := map[string]*UnitChange{}
	for _, g := range groups {
		if u, ok := inCatalog[g.Template]; ok {
			rep.Unchanged++
			if u.Korean == "" && g.Class == inventory.ClassProse {
				rep.Untranslated++
			}
			continue
		}
		newByTemplate[g.Template] = &UnitChange{
			Kind: KindNew, Template: g.Template, Rep: g.Rep,
			Class: string(g.Class), Members: len(g.Members),
		}
	}

	var orphans []UnitChange
	for _, u := range cat.Units {
		if _, ok := current[u.Template]; ok {
			continue
		}
		orphans = append(orphans, UnitChange{
			Kind: KindOrphaned, Template: u.Template, Rep: u.Rep,
			Class: u.Class, Members: u.Members, PreviousKorean: u.Korean,
		})
	}

	// Pair each orphan with the template that now covers its representative
	// key. That pairing is what turns a confusing "one string vanished and
	// another appeared" into "this string was reworded".
	for _, o := range orphans {
		tmpl, ok := templateOfKey[o.Rep]
		if !ok {
			rep.Orphaned = append(rep.Orphaned, o)
			continue
		}
		n, isNew := newByTemplate[tmpl]
		if !isNew {
			// The key survives but now shares an existing template, so there
			// is nothing to translate; the orphan is simply dead.
			rep.Orphaned = append(rep.Orphaned, o)
			continue
		}
		rep.Changed = append(rep.Changed, UnitChange{
			Kind: KindChanged, Template: n.Template, Rep: n.Rep,
			Class: n.Class, Members: n.Members,
			PreviousTemplate: o.Template, PreviousKorean: o.PreviousKorean,
		})
		delete(newByTemplate, tmpl)
	}

	for _, n := range newByTemplate {
		rep.New = append(rep.New, *n)
	}

	sortChanges(rep.New)
	sortChanges(rep.Orphaned)
	sortChanges(rep.Changed)

	if pack != nil {
		rep.PackCompared = true
		built := make(map[string]struct{}, len(pack.Entries))
		for _, e := range pack.Entries {
			built[e.Key] = struct{}{}
		}
		source := make(map[string]struct{}, len(entries))
		for _, e := range entries {
			source[e.Key] = struct{}{}
			if _, ok := built[e.Key]; !ok {
				rep.AddedKeys = append(rep.AddedKeys, e.Key)
			}
		}
		for _, e := range pack.Entries {
			if _, ok := source[e.Key]; !ok {
				rep.RemovedKeys = append(rep.RemovedKeys, e.Key)
			}
		}
		sort.Strings(rep.AddedKeys)
		sort.Strings(rep.RemovedKeys)
	}

	return rep
}

func sortChanges(cs []UnitChange) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Members != cs[j].Members {
			return cs[i].Members > cs[j].Members
		}
		return cs[i].Rep < cs[j].Rep
	})
}

// MergeStats reports what Merge did to a catalog.
type MergeStats struct {
	Added   int `json:"added"`
	Kept    int `json:"kept"`
	Pruned  int `json:"pruned"`
	Refresh int `json:"refreshed"`
}

// Merge brings a catalog up to date with the mod's current groups, preserving
// every translation whose English text is unchanged. This is what makes an
// IV2 update cost only the strings that actually moved.
func Merge(cat *build.Catalog, groups []inventory.Group, prune bool) MergeStats {
	var st MergeStats

	existing := make(map[string]build.Unit, len(cat.Units))
	for _, u := range cat.Units {
		existing[u.Template] = u
	}
	current := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		current[g.Template] = struct{}{}
	}

	units := make([]build.Unit, 0, len(groups))
	for _, g := range groups {
		if u, ok := existing[g.Template]; ok {
			// Carry the translation over, but refresh the metadata: a
			// surviving string can gain or lose members across an update.
			if u.Rep != g.Rep || u.Members != len(g.Members) || u.Class != string(g.Class) {
				st.Refresh++
			}
			u.Rep, u.Members, u.Class = g.Rep, len(g.Members), string(g.Class)
			units = append(units, u)
			st.Kept++
			continue
		}
		units = append(units, build.Unit{
			Template: g.Template, Rep: g.Rep,
			Class: string(g.Class), Members: len(g.Members),
		})
		st.Added++
	}

	// Orphans are kept by default: a string that vanished in one version can
	// come back in the next, and its translation is worth more than the bytes.
	for _, u := range cat.Units {
		if _, ok := current[u.Template]; ok {
			continue
		}
		if prune {
			st.Pruned++
			continue
		}
		units = append(units, u)
	}

	cat.Units = units
	cat.Sort()
	return st
}
