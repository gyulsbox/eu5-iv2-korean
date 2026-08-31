package inventory

import (
	"testing"
)

const fixture = "testdata/mod"

func TestScanFindsOnlySourceLanguage(t *testing.T) {
	inv, err := Scan(fixture, "english")
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Files) != 3 {
		t.Fatalf("got %d files, want 3: %+v", len(inv.Files), inv.Files)
	}
	for _, f := range inv.Files {
		if f.Language != "l_english" {
			t.Errorf("%s: language = %q", f.Path, f.Language)
		}
		if f.Layer != "main_menu" {
			t.Errorf("%s: layer = %q, want main_menu", f.Path, f.Layer)
		}
	}
}

func TestScanReportsMissingBOM(t *testing.T) {
	inv, err := Scan(fixture, "english")
	if err != nil {
		t.Fatal(err)
	}
	var withBOM, withoutBOM int
	for _, f := range inv.Files {
		if f.HadBOM {
			withBOM++
		} else {
			withoutBOM++
		}
	}
	if withBOM != 2 || withoutBOM != 1 {
		t.Errorf("BOM tally = %d with / %d without, want 2/1", withBOM, withoutBOM)
	}
}

func TestScanRecordsDuplicateKeys(t *testing.T) {
	inv, err := Scan(fixture, "english")
	if err != nil {
		t.Fatal(err)
	}
	locs, ok := inv.Duplicates["iv2_level_1"]
	if !ok {
		t.Fatalf("iv2_level_1 not reported as duplicate; duplicates = %v", inv.Duplicates)
	}
	if len(locs) != 2 {
		t.Errorf("iv2_level_1 recorded at %v, want 2 locations", locs)
	}
}

func TestScanRecoversMalformedEntry(t *testing.T) {
	inv, err := Scan(fixture, "english")
	if err != nil {
		t.Fatal(err)
	}
	var malformed int
	var found bool
	for _, e := range inv.Entries {
		if e.Malformed {
			malformed++
		}
		if e.Key == "iv2_appoint_researcher_adm_act_past" {
			found = true
			if !e.Malformed {
				t.Error("unterminated entry not flagged")
			}
		}
	}
	if !found {
		t.Error("recovered entry missing from the inventory")
	}
	if malformed != 1 {
		t.Errorf("malformed count = %d, want 1", malformed)
	}
}

func TestScanEndToEndStats(t *testing.T) {
	inv, err := Scan(fixture, "english")
	if err != nil {
		t.Fatal(err)
	}
	groups := Groups(inv.Entries)
	s := Summarize(inv.Entries, groups)

	if s.TotalKeys != len(inv.Entries) {
		t.Errorf("TotalKeys = %d, want %d", s.TotalKeys, len(inv.Entries))
	}
	// Grouping must never invent or lose a key.
	var members int
	for _, g := range groups {
		members += len(g.Members)
	}
	if members != s.TotalKeys {
		t.Errorf("groups cover %d keys, want %d", members, s.TotalKeys)
	}
	// Every member must reconstruct its own source value byte for byte,
	// otherwise the build stage would emit corrupt localization.
	byKey := map[string]string{}
	for _, e := range inv.Entries {
		byKey[e.Key] = e.Value
	}
	for _, g := range groups {
		for _, m := range g.Members {
			got, ok := Detemplatize(g.Template, m.Tokens, m.Numbers)
			if !ok {
				t.Errorf("%s: Detemplatize reported a bad index", m.Key)
				continue
			}
			// A duplicate key legitimately maps to one of its definitions.
			if got != byKey[m.Key] {
				t.Errorf("%s reconstructed as %q, want %q", m.Key, got, byKey[m.Key])
			}
		}
	}
	if s.ProseGroups >= s.TotalKeys {
		t.Errorf("grouping achieved no reduction: %d prose groups vs %d keys", s.ProseGroups, s.TotalKeys)
	}
}

func TestScanNoMatchingFiles(t *testing.T) {
	inv, err := Scan(fixture, "korean")
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Files) != 0 {
		t.Errorf("got %d files for a language with none, want 0", len(inv.Files))
	}
}
