package validate

import (
	"reflect"
	"strings"
	"testing"
)

func TestReadKeyDigest(t *testing.T) {
	src := KeyDigestHeader + `
# source: C:/…/game
main_menu	cw_settings_shared
in_game	ADD_IDEAGROUP_SLOT_TOOLTIP_ADM

dlc	DLC_ONLY_KEY
`
	got, err := ReadKeyDigest(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []KeyRef{
		{Layer: "main_menu", Key: "cw_settings_shared"},
		{Layer: "in_game", Key: "ADD_IDEAGROUP_SLOT_TOOLTIP_ADM"},
		{Layer: "dlc", Key: "DLC_ONLY_KEY"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadKeyDigest = %+v, want %+v", got, want)
	}
}

func TestReadKeyDigestAcceptsBareKeyList(t *testing.T) {
	// A hand-written list, or one produced by grep, should work too.
	got, err := ReadKeyDigest(strings.NewReader("first_key\n  second_key  \n\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []KeyRef{{Key: "first_key"}, {Key: "second_key"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadKeyDigest = %+v, want %+v", got, want)
	}
}

func TestReadKeyDigestHandlesCRLF(t *testing.T) {
	// The digest will typically be produced on Windows.
	got, err := ReadKeyDigest(strings.NewReader("main_menu\tkey_one\r\nin_game\tkey_two\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Key != "key_one" || got[1].Layer != "in_game" {
		t.Errorf("ReadKeyDigest = %+v", got)
	}
}

func TestDigestBaselineCatchesTheSameCollisions(t *testing.T) {
	// A digest must carry exactly the authority of the tree it came from.
	fromTree, err := Run(Options{Source: fixtureSource, Baselines: []string{fixtureBaseline}})
	if err != nil {
		t.Fatal(err)
	}

	digest := t.TempDir() + "/basegame.keys"
	writeDigestFor(t, fromTree, digest)

	fromDigest, err := Run(Options{Source: fixtureSource, BaselineDigests: []string{digest}})
	if err != nil {
		t.Fatal(err)
	}

	if !fromDigest.Report.BaselineChecked {
		t.Error("a digest baseline did not count as a check")
	}
	if !reflect.DeepEqual(fromTree.Shadowed, fromDigest.Shadowed) {
		t.Errorf("digest found %v, tree found %v", fromDigest.Shadowed, fromTree.Shadowed)
	}
	for _, k := range fromDigest.Shadowed {
		if got, want := fromDigest.BaselineKeys[k].Layer, fromTree.BaselineKeys[k].Layer; got != want {
			t.Errorf("%s layer via digest = %q, want %q", k, got, want)
		}
	}
}

func writeDigestFor(t *testing.T, res *Result, path string) {
	t.Helper()
	var b strings.Builder
	b.WriteString(KeyDigestHeader + "\n")
	for key, ref := range res.BaselineKeys {
		b.WriteString(ref.Layer + "\t" + key + "\n")
	}
	writeFile(t, path, b.String())
}
