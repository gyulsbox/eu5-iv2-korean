package diff

import (
	"testing"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/build"
	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/inventory"
)

// entries builds a source inventory from key/value pairs.
func entries(kv ...string) []inventory.Entry {
	var out []inventory.Entry
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, inventory.Entry{Key: kv[i], Value: kv[i+1], Layer: "main_menu"})
	}
	return out
}

// catalogFor builds a translated catalog from the given source.
func catalogFor(es []inventory.Entry, korean map[string]string) *build.Catalog {
	cat := build.NewCatalog(inventory.Groups(es))
	for i, u := range cat.Units {
		if ko, ok := korean[u.Template]; ok {
			cat.Units[i].Korean = ko
		}
	}
	return cat
}

func findChange(cs []UnitChange, rep string) (UnitChange, bool) {
	for _, c := range cs {
		if c.Rep == rep {
			return c, true
		}
	}
	return UnitChange{}, false
}

func TestCompareNoChanges(t *testing.T) {
	src := entries("a", "Innovativeness Ideas", "b", "Level 1")
	cat := catalogFor(src, map[string]string{
		"Innovativeness Ideas": "혁신 이념",
		"Level ⟦N1⟧":           "레벨 ⟦N1⟧",
	})

	rep := Compare(inventory.Groups(src), src, cat, nil)
	if rep.Work() != 0 {
		t.Errorf("Work = %d on an unchanged mod: new %d, changed %d, untranslated %d",
			rep.Work(), len(rep.New), len(rep.Changed), rep.Untranslated)
	}
	if rep.Unchanged != 2 {
		t.Errorf("Unchanged = %d, want 2", rep.Unchanged)
	}
}

func TestCompareDetectsNewString(t *testing.T) {
	before := entries("a", "Innovativeness Ideas")
	cat := catalogFor(before, map[string]string{"Innovativeness Ideas": "혁신 이념"})

	after := entries("a", "Innovativeness Ideas", "b", "Patron of the Arts")
	rep := Compare(inventory.Groups(after), after, cat, nil)

	if len(rep.New) != 1 || rep.New[0].Rep != "b" {
		t.Fatalf("New = %+v, want one entry for b", rep.New)
	}
	if len(rep.Orphaned) != 0 || len(rep.Changed) != 0 {
		t.Errorf("a genuinely new string produced orphaned=%d changed=%d", len(rep.Orphaned), len(rep.Changed))
	}
}

func TestCompareDetectsRewording(t *testing.T) {
	// The case that matters most: IV2 rewrites a string. Reported as one
	// "changed" rather than an unrelated add and remove, and it carries the
	// old Korean so a small reword is cheap to adapt.
	before := entries("a", "Innovativeness Ideas")
	cat := catalogFor(before, map[string]string{"Innovativeness Ideas": "혁신 이념"})

	after := entries("a", "Innovativeness Ideas Reworked")
	rep := Compare(inventory.Groups(after), after, cat, nil)

	if len(rep.Changed) != 1 {
		t.Fatalf("Changed = %+v, want one entry", rep.Changed)
	}
	c := rep.Changed[0]
	if c.Rep != "a" {
		t.Errorf("Rep = %q, want a", c.Rep)
	}
	if c.Template != "Innovativeness Ideas Reworked" {
		t.Errorf("Template = %q", c.Template)
	}
	if c.PreviousTemplate != "Innovativeness Ideas" {
		t.Errorf("PreviousTemplate = %q", c.PreviousTemplate)
	}
	if c.PreviousKorean != "혁신 이념" {
		t.Errorf("PreviousKorean = %q, want the old translation", c.PreviousKorean)
	}
	// A reword must not also be counted as a separate add and remove.
	if len(rep.New) != 0 || len(rep.Orphaned) != 0 {
		t.Errorf("reword double-counted: new=%+v orphaned=%+v", rep.New, rep.Orphaned)
	}
}

func TestCompareDetectsRemovedString(t *testing.T) {
	before := entries("a", "Innovativeness Ideas", "b", "Patron of the Arts")
	cat := catalogFor(before, map[string]string{
		"Innovativeness Ideas": "혁신 이념",
		"Patron of the Arts":   "예술의 후원자",
	})

	after := entries("a", "Innovativeness Ideas")
	rep := Compare(inventory.Groups(after), after, cat, nil)

	if len(rep.Orphaned) != 1 || rep.Orphaned[0].Rep != "b" {
		t.Fatalf("Orphaned = %+v, want one entry for b", rep.Orphaned)
	}
	if len(rep.Changed) != 0 {
		t.Errorf("Changed = %+v, want none", rep.Changed)
	}
}

func TestCompareCountsUntranslatedSurvivors(t *testing.T) {
	src := entries("a", "Innovativeness Ideas", "b", "Patron of the Arts")
	cat := catalogFor(src, map[string]string{"Innovativeness Ideas": "혁신 이념"})

	rep := Compare(inventory.Groups(src), src, cat, nil)
	if rep.Untranslated != 1 {
		t.Errorf("Untranslated = %d, want 1", rep.Untranslated)
	}
	if rep.Work() != 1 {
		t.Errorf("Work = %d, want 1", rep.Work())
	}
}

func TestCompareKeyLevelAgainstPack(t *testing.T) {
	before := entries("a", "One", "b", "Two")
	cat := catalogFor(before, nil)
	pack := &inventory.Inventory{Entries: entries("a", "하나", "b", "둘")}

	after := entries("a", "One", "c", "Three")
	rep := Compare(inventory.Groups(after), after, cat, pack)

	if !rep.PackCompared {
		t.Error("PackCompared false with a pack supplied")
	}
	if len(rep.AddedKeys) != 1 || rep.AddedKeys[0] != "c" {
		t.Errorf("AddedKeys = %v, want [c]", rep.AddedKeys)
	}
	if len(rep.RemovedKeys) != 1 || rep.RemovedKeys[0] != "b" {
		t.Errorf("RemovedKeys = %v, want [b]", rep.RemovedKeys)
	}
}

func TestMergePreservesTranslationsAcrossAnUpdate(t *testing.T) {
	// The property the whole update story rests on: a string whose English is
	// unchanged keeps its Korean, so an IV2 update costs only what moved.
	before := entries("a", "Innovativeness Ideas", "b", "Patron of the Arts")
	cat := catalogFor(before, map[string]string{
		"Innovativeness Ideas": "혁신 이념",
		"Patron of the Arts":   "예술의 후원자",
	})

	after := entries(
		"a", "Innovativeness Ideas", // unchanged
		"b", "Patron of the Arts Reworded", // reworded
		"c", "Pragmatism", // new
	)
	st := Merge(cat, inventory.Groups(after), false)

	byTemplate := map[string]string{}
	for _, u := range cat.Units {
		byTemplate[u.Template] = u.Korean
	}
	if byTemplate["Innovativeness Ideas"] != "혁신 이념" {
		t.Error("an unchanged string lost its translation")
	}
	if byTemplate["Patron of the Arts Reworded"] != "" {
		t.Error("a reworded string kept stale Korean for new English")
	}
	if byTemplate["Pragmatism"] != "" {
		t.Error("a new string arrived pre-translated")
	}
	// The orphan is kept by default so its Korean survives a temporary removal.
	if _, ok := byTemplate["Patron of the Arts"]; !ok {
		t.Error("orphan dropped without --prune")
	}
	if st.Kept != 1 || st.Added != 2 {
		t.Errorf("MergeStats = %+v, want kept 1 added 2", st)
	}
}

func TestMergePrune(t *testing.T) {
	before := entries("a", "One", "b", "Two")
	cat := catalogFor(before, map[string]string{"One": "하나", "Two": "둘"})

	after := entries("a", "One")
	st := Merge(cat, inventory.Groups(after), true)

	if st.Pruned != 1 {
		t.Errorf("Pruned = %d, want 1", st.Pruned)
	}
	if len(cat.Units) != 1 {
		t.Errorf("catalog holds %d units after pruning, want 1", len(cat.Units))
	}
	if cat.Units[0].Korean != "하나" {
		t.Errorf("surviving unit lost its translation: %+v", cat.Units[0])
	}
}

func TestMergeRefreshesMemberCounts(t *testing.T) {
	// A surviving string can gain keys across an update; the catalog's
	// members count is what tells a reviewer the blast radius of an edit.
	before := entries("a", "Level 1")
	cat := catalogFor(before, map[string]string{"Level ⟦N1⟧": "레벨 ⟦N1⟧"})

	after := entries("a", "Level 1", "b", "Level 2", "c", "Level 3")
	st := Merge(cat, inventory.Groups(after), false)

	if len(cat.Units) != 1 {
		t.Fatalf("got %d units, want 1", len(cat.Units))
	}
	if cat.Units[0].Members != 3 {
		t.Errorf("Members = %d, want 3", cat.Units[0].Members)
	}
	if cat.Units[0].Korean != "레벨 ⟦N1⟧" {
		t.Error("refresh dropped the translation")
	}
	if st.Refresh != 1 {
		t.Errorf("Refresh = %d, want 1", st.Refresh)
	}
}

func TestMergeIsIdempotent(t *testing.T) {
	src := entries("a", "One", "b", "Two")
	cat := catalogFor(src, map[string]string{"One": "하나"})
	groups := inventory.Groups(src)

	Merge(cat, groups, false)
	first := len(cat.Units)
	st := Merge(cat, groups, false)

	if len(cat.Units) != first {
		t.Errorf("second merge changed unit count: %d -> %d", first, len(cat.Units))
	}
	if st.Added != 0 || st.Pruned != 0 {
		t.Errorf("second merge was not a no-op: %+v", st)
	}
}
