package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/inventory"
	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/paradox"
)

const fixture = "../validate/testdata/source"

func buildInto(t *testing.T, cat *Catalog, opts ...func(*Options)) (string, *Stats) {
	t.Helper()
	dir := t.TempDir()
	o := Options{Source: fixture, Out: dir, Catalog: cat, Meta: DefaultMetadata()}
	for _, f := range opts {
		f(&o)
	}
	stats, err := Run(o)
	if err != nil {
		t.Fatal(err)
	}
	return dir, stats
}

// readBuilt parses everything the build wrote back in.
func readBuilt(t *testing.T, dir string) map[string]string {
	t.Helper()
	inv, err := inventory.Scan(dir, "korean")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range inv.Entries {
		out[e.Key] = e.Value
	}
	return out
}

func sourceValues(t *testing.T) map[string]string {
	t.Helper()
	inv, err := inventory.Scan(fixture, "english")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range inv.Entries {
		out[e.Key] = e.Value
	}
	return out
}

func TestBuildWithoutCatalogReproducesSourceExactly(t *testing.T) {
	// The passthrough build is how the pipeline gets verified before any
	// translation exists, so it has to be lossless.
	dir, stats := buildInto(t, nil)
	src := sourceValues(t)
	got := readBuilt(t, dir)

	if len(got) != len(src) {
		t.Fatalf("built %d keys, source has %d", len(got), len(src))
	}
	for k, want := range src {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
	if stats.Translated != 0 {
		t.Errorf("Translated = %d on a passthrough build, want 0", stats.Translated)
	}
	if stats.English != len(src) {
		t.Errorf("English = %d, want %d", stats.English, len(src))
	}
}

func TestBuildWritesBOMOnEveryFile(t *testing.T) {
	// Without the BOM, EU5 skips the file and says nothing at all, so this is
	// checked at the byte level rather than through the parser.
	dir, _ := buildInto(t, nil)
	var checked int
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.HasSuffix(p, ".yml") {
			return err
		}
		checked++
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(string(data), paradox.BOM) {
			t.Errorf("%s does not start with a UTF-8 BOM", p)
		}
		if got := data[:3]; got[0] != 0xEF || got[1] != 0xBB || got[2] != 0xBF {
			t.Errorf("%s starts with % x, want ef bb bf", p, got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no .yml files were written")
	}
}

func TestBuildWritesLanguageHeader(t *testing.T) {
	// A header of `korean:` instead of `l_korean:` parses as nothing and the
	// game reads no keys from the file, while every other check still passes.
	dir, _ := buildInto(t, nil)
	inv, err := inventory.Scan(dir, "korean")
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Files) == 0 {
		t.Fatal("no files written")
	}
	for _, f := range inv.Files {
		if f.Language != "l_korean" {
			t.Errorf("%s header = %q, want l_korean", f.Path, f.Language)
		}
	}
}

func TestBuildMirrorsSourceLayerAndOrdersLast(t *testing.T) {
	dir, _ := buildInto(t, nil)
	inv, err := inventory.Scan(dir, "korean")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range inv.Files {
		// IV2 keeps everything under main_menu; a file elsewhere is never read.
		if f.Layer != "main_menu" {
			t.Errorf("%s is in layer %q, want main_menu", f.Path, f.Layer)
		}
		if !strings.Contains(f.Path, "/localization/korean/") {
			t.Errorf("%s is not under localization/korean", f.Path)
		}
		if !strings.HasPrefix(filepath.Base(f.Path), LoadOrderPrefix) {
			t.Errorf("%s lacks the %q load-order prefix", f.Path, LoadOrderPrefix)
		}
	}
}

func TestBuildAppliesTranslations(t *testing.T) {
	inv, err := inventory.Scan(fixture, "english")
	if err != nil {
		t.Fatal(err)
	}
	groups := inventory.Groups(inv.Entries)
	cat := NewCatalog(groups)

	// Translate the one group covering iv2_level_1..3, which differ only in
	// their number, and one plain string.
	for i := range cat.Units {
		switch cat.Units[i].Template {
		case "Level ⟦N1⟧":
			cat.Units[i].Korean = "레벨 ⟦N1⟧"
		case "Innovativeness Ideas":
			cat.Units[i].Korean = "혁신 이념"
		}
	}

	dir, stats := buildInto(t, cat)
	got := readBuilt(t, dir)

	if got["iv2_ideagroup_title_adm_1"] != "혁신 이념" {
		t.Errorf("plain translation not applied: %q", got["iv2_ideagroup_title_adm_1"])
	}
	// One translated template must fill in each member's own number.
	if got["iv2_level_1"] != "레벨 1" {
		t.Errorf("iv2_level_1 = %q, want 레벨 1", got["iv2_level_1"])
	}
	if got["iv2_level_3"] != "레벨 3" {
		t.Errorf("iv2_level_3 = %q, want 레벨 3", got["iv2_level_3"])
	}
	if stats.Translated < 4 {
		t.Errorf("Translated = %d, want at least 4", stats.Translated)
	}
	if stats.FellBack != 0 {
		t.Errorf("FellBack = %d on sound translations: %v", stats.FellBack, stats.FallbackKeys)
	}
}

func TestBuildFallsBackOnBrokenTranslation(t *testing.T) {
	inv, err := inventory.Scan(fixture, "english")
	if err != nil {
		t.Fatal(err)
	}
	cat := NewCatalog(inventory.Groups(inv.Entries))

	src := sourceValues(t)
	broken := "Researchers cost [iv2_researcher_upkeep|e] each month.\\nThey help."
	var patched bool
	for i := range cat.Units {
		if cat.Units[i].Rep == "iv2_desc" {
			// Drops the scope token and the newline: exactly the damage that
			// leaves a tooltip unable to render.
			cat.Units[i].Korean = "연구원은 매달 비용이 듭니다."
			patched = true
		}
	}
	if !patched {
		t.Fatalf("fixture changed: no unit for iv2_desc")
	}

	dir, stats := buildInto(t, cat)
	got := readBuilt(t, dir)

	if got["iv2_desc"] != src["iv2_desc"] {
		t.Errorf("iv2_desc = %q, want the English source %q", got["iv2_desc"], src["iv2_desc"])
	}
	if src["iv2_desc"] != broken {
		t.Logf("fixture source is %q", src["iv2_desc"])
	}
	if stats.FellBack != 1 {
		t.Errorf("FellBack = %d, want 1", stats.FellBack)
	}
	if len(stats.FallbackKeys) != 1 || stats.FallbackKeys[0] != "iv2_desc" {
		t.Errorf("FallbackKeys = %v, want [iv2_desc]", stats.FallbackKeys)
	}
}

func TestBuildNeverTranslatesEngineTokens(t *testing.T) {
	inv, err := inventory.Scan(fixture, "english")
	if err != nil {
		t.Fatal(err)
	}
	cat := NewCatalog(inventory.Groups(inv.Entries))
	for i := range cat.Units {
		if cat.Units[i].Template == "green" {
			cat.Units[i].Korean = "초록"
		}
	}

	dir, stats := buildInto(t, cat)
	got := readBuilt(t, dir)

	// Even with a translation on offer, the colour must ship verbatim.
	if got["iv2_idea_alert_adm_color"] != "green" {
		t.Errorf("colour token = %q, want green", got["iv2_idea_alert_adm_color"])
	}
	if stats.DoNotTouch == 0 {
		t.Error("DoNotTouch = 0, want the colour key counted")
	}
}

func TestBuildRecoversMalformedSourceIntoValidOutput(t *testing.T) {
	// IV2 ships values with no closing quote. The generated file must be
	// well-formed even though its source was not.
	dir, _ := buildInto(t, nil)
	got := readBuilt(t, dir)
	v, ok := got["iv2_broken_source"]
	if !ok {
		t.Fatal("recovered key missing from the build")
	}
	if v != "[Scope.GetName] was appointed." {
		t.Errorf("recovered value = %q", v)
	}
}

func TestBuildWritesMetadataAndThumbnail(t *testing.T) {
	dir, _ := buildInto(t, nil)

	data, err := os.ReadFile(filepath.Join(dir, ".metadata", "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), paradox.BOM) {
		t.Error("metadata.json has no BOM; IV2's own carries one")
	}
	body := strings.TrimPrefix(string(data), paradox.BOM)
	// The previous pack was flagged for declaring a game version it did not
	// match; the declared version must track the game.
	for _, want := range []string{`"supported_game_version": "1.3.*"`, `"id": "iv2_korean"`, `"Idea_Variation_2"`} {
		if !strings.Contains(body, want) {
			t.Errorf("metadata.json missing %s\n%s", want, body)
		}
	}

	png, err := os.ReadFile(filepath.Join(dir, ".metadata", "thumbnail.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(png) < 8 || string(png[1:4]) != "PNG" {
		t.Error("thumbnail.png is not a PNG")
	}
}

func TestBuildKeepsAnExistingThumbnail(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".metadata"), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(dir, ".metadata", "thumbnail.png")
	if err := os.WriteFile(custom, []byte("real artwork goes here"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(Options{Source: fixture, Out: dir, Meta: DefaultMetadata()}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(custom)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "real artwork goes here" {
		t.Error("build overwrote a thumbnail the user supplied")
	}
}

func TestBuildSingleFile(t *testing.T) {
	dir, _ := buildInto(t, nil, func(o *Options) { o.SingleFile = true })
	inv, err := inventory.Scan(dir, "korean")
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Files) != 1 {
		t.Fatalf("single-file build wrote %d files", len(inv.Files))
	}
	if got := filepath.Base(inv.Files[0].Path); got != "0_iv2_korean_l_korean.yml" {
		t.Errorf("file name = %q", got)
	}
	if len(inv.Entries) != len(sourceValues(t)) {
		t.Errorf("single file holds %d keys, want %d", len(inv.Entries), len(sourceValues(t)))
	}
}

func TestCatalogRoundTrip(t *testing.T) {
	inv, err := inventory.Scan(fixture, "english")
	if err != nil {
		t.Fatal(err)
	}
	cat := NewCatalog(inventory.Groups(inv.Entries))
	cat.Units[0].Korean = "번역"
	cat.Sort()

	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := WriteCatalog(path, cat); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Units) != len(cat.Units) {
		t.Errorf("round trip lost units: %d vs %d", len(got.Units), len(cat.Units))
	}
	if got.Translated() != 1 {
		t.Errorf("Translated = %d, want 1", got.Translated())
	}
}

func TestReadCatalogRejectsWrongVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, []byte(`{"version":999,"units":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCatalog(path); err == nil {
		t.Error("ReadCatalog accepted an unknown version")
	}
}
