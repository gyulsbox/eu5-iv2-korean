package translate

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/gyulsbox/eu5-iv2-korean/internal/build"
	"github.com/gyulsbox/eu5-iv2-korean/internal/inventory"
)

func TestCheckPlaceholdersAcceptsReordering(t *testing.T) {
	// Korean word order moves the placeholder; that is the whole reason they
	// are numbered, so it must not be treated as damage.
	cases := []struct{ src, ko string }{
		{"⟦T1⟧ selected ⟦T2⟧", "⟦T2⟧을(를) ⟦T1⟧"},
		{"Level ⟦N1⟧", "레벨 ⟦N1⟧"},
		{"Costs ⟦T1⟧ ⟦T2⟧⟦T3⟧ to unlock.", "해금하려면 ⟦T1⟧ ⟦T2⟧⟦T3⟧ 필요."},
		{"no placeholders here", "여기엔 자리표시자가 없습니다"},
		{"", ""},
	}
	for _, c := range cases {
		if reason := checkPlaceholders(c.src, c.ko); reason != "" {
			t.Errorf("checkPlaceholders(%q, %q) = %q, want accepted", c.src, c.ko, reason)
		}
	}
}

func TestCheckPlaceholdersCatchesDamage(t *testing.T) {
	cases := []struct{ name, src, ko, want string }{
		{"dropped", "⟦T1⟧ of ⟦T2⟧", "⟦T1⟧의 것", "lost"},
		{"invented", "Level ⟦N1⟧", "레벨 ⟦N1⟧ / ⟦N2⟧", "added"},
		{"renumbered", "⟦T1⟧ ⟦T2⟧", "⟦T1⟧ ⟦T3⟧", "lost"},
		{"duplicated", "⟦N1⟧%", "⟦N1⟧⟦N1⟧%", "added"},
		{"translated the letter", "⟦T1⟧", "⟦ㅌ1⟧", "lost"},
		{"blanked", "Innovativeness Ideas", "", "empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason := checkPlaceholders(c.src, c.ko)
			if reason == "" {
				t.Fatalf("checkPlaceholders(%q, %q) accepted damage", c.src, c.ko)
			}
			if !strings.Contains(reason, c.want) {
				t.Errorf("reason = %q, want it to mention %q", reason, c.want)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	want := map[string]string{"1": "혁신 이념", "2": "레벨 ⟦N1⟧"}
	cases := []struct{ name, body string }{
		{"bare json", `{"1": "혁신 이념", "2": "레벨 ⟦N1⟧"}`},
		{"markdown fenced", "```json\n{\"1\": \"혁신 이념\", \"2\": \"레벨 ⟦N1⟧\"}\n```"},
		{"unlabelled fence", "```\n{\"1\": \"혁신 이념\", \"2\": \"레벨 ⟦N1⟧\"}\n```"},
		{"with preamble", "Here are the translations:\n{\"1\": \"혁신 이념\", \"2\": \"레벨 ⟦N1⟧\"}"},
		{"with trailing prose", `{"1": "혁신 이념", "2": "레벨 ⟦N1⟧"}` + "\nLet me know if you need changes."},
		{"leading whitespace", "\n\n  {\"1\": \"혁신 이념\", \"2\": \"레벨 ⟦N1⟧\"}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseResponse(c.body)
			if err != nil {
				t.Fatalf("parseResponse: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestParseResponseRejectsGarbage(t *testing.T) {
	for _, body := range []string{"", "I cannot translate this.", "{not json}", "[1,2,3]"} {
		if _, err := parseResponse(body); err == nil {
			t.Errorf("parseResponse(%q) accepted garbage", body)
		}
	}
}

func TestPendingSkipsNonProseAndDone(t *testing.T) {
	cat := &build.Catalog{Version: build.CatalogVersion, Units: []build.Unit{
		{Template: "Innovativeness Ideas", Class: string(inventory.ClassProse)},
		{Template: "⟦T1⟧ ⟦T2⟧", Class: string(inventory.ClassReference)},
		{Template: "", Class: string(inventory.ClassEmpty)},
		{Template: "Level ⟦N1⟧", Class: string(inventory.ClassProse), Korean: "레벨 ⟦N1⟧"},
		{Template: "Patron of the Arts", Class: string(inventory.ClassProse)},
	}}

	got := Pending(cat, false)
	if len(got) != 2 {
		t.Fatalf("Pending returned %d items, want 2: %+v", len(got), got)
	}
	if got[0].Source != "Innovativeness Ideas" || got[1].Source != "Patron of the Arts" {
		t.Errorf("Pending returned %+v", got)
	}
	// Index must point back at the right catalog slot or translations land on
	// the wrong unit.
	if got[1].Index != 4 {
		t.Errorf("second item Index = %d, want 4", got[1].Index)
	}

	if n := len(Pending(cat, true)); n != 3 {
		t.Errorf("Pending(retranslate) returned %d, want 3", n)
	}
}

func TestChunk(t *testing.T) {
	items := make([]Item, 10)
	got := chunk(items, 4)
	if len(got) != 3 {
		t.Fatalf("chunk produced %d batches, want 3", len(got))
	}
	if len(got[0]) != 4 || len(got[2]) != 2 {
		t.Errorf("batch sizes = %d, %d, %d", len(got[0]), len(got[1]), len(got[2]))
	}

	// Batches must not alias each other, since each is assigned its own ids.
	a := chunk(make([]Item, 4), 2)
	a[0][0].ID = "x"
	if a[1][0].ID == "x" {
		t.Error("batches share backing storage")
	}
}

func TestGlossaryRelevant(t *testing.T) {
	g := &Glossary{Terms: map[string]string{
		"Idea Group":  "이념 그룹",
		"Navy":        "해군",
		"Casus Belli": "명분",
	}}
	got := g.Relevant([]string{"Unlock a new Navy Ideagroup Slot.", "Can Create ⟦T1⟧ Casus Belli."})
	if _, ok := got["Navy"]; !ok {
		t.Error("Navy not matched")
	}
	if _, ok := got["Casus Belli"]; !ok {
		t.Error("Casus Belli not matched")
	}
	// Sending terms a batch cannot use wastes prompt and dilutes attention.
	if _, ok := got["Idea Group"]; ok {
		t.Error("Idea Group matched though absent from the batch")
	}
}

func TestGlossaryRelevantNilSafe(t *testing.T) {
	var g *Glossary
	if got := g.Relevant([]string{"anything"}); got != nil {
		t.Errorf("nil glossary returned %v", got)
	}
}

func TestBuildUserMessageCarriesEveryItem(t *testing.T) {
	items := []Item{{ID: "1", Source: "Innovativeness Ideas"}, {ID: "2", Source: "Level ⟦N1⟧"}}
	msg := BuildUserMessage(items, map[string]string{"Idea": "이념"})
	for _, want := range []string{`"1"`, `"2"`, "Innovativeness Ideas", "Level ⟦N1⟧", "Idea -> 이념"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestRetryable(t *testing.T) {
	retry := []int{408, 409, 429, 500, 502, 503, 529}
	for _, code := range retry {
		err := &anthropic.Error{StatusCode: code, Response: &http.Response{StatusCode: code}}
		if !retryable(err) {
			t.Errorf("status %d judged non-retryable", code)
		}
	}
	// A bad request will fail identically every time; retrying burns quota.
	for _, code := range []int{400, 401, 403, 404, 422} {
		err := &anthropic.Error{StatusCode: code, Response: &http.Response{StatusCode: code}}
		if retryable(err) {
			t.Errorf("status %d judged retryable", code)
		}
	}
	// Connection failures carry no status and are worth another attempt.
	if !retryable(errors.New("connection reset by peer")) {
		t.Error("connection error judged non-retryable")
	}
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	prev := backoff(1)
	for attempt := 2; attempt <= 5; attempt++ {
		d := backoff(attempt)
		if d < prev {
			t.Errorf("backoff(%d) = %v, shorter than backoff(%d) = %v", attempt, d, attempt-1, prev)
		}
		prev = d
	}
	if d := backoff(20); d > 61*1e9 {
		t.Errorf("backoff(20) = %v, want it capped near a minute", d)
	}
}

func TestStatsCost(t *testing.T) {
	// Haiku 4.5 list price: $1 per Mtok in, $5 per Mtok out.
	s := Stats{InTokens: 1_000_000, OutTokens: 1_000_000}
	if got := s.Cost(); got < 5.99 || got > 6.01 {
		t.Errorf("Cost() = %v, want 6.00", got)
	}
}
