// Package translate fills a build catalog's Korean fields using the Claude API.
package translate

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Glossary fixes the rendering of terms that must stay consistent across the
// whole pack. Without it, "Idea Group" comes back as 이념 그룹 in one tooltip
// and 아이디어 그룹 in the next.
type Glossary struct {
	Terms map[string]string `json:"terms"`
}

// ReadGlossary loads a glossary file.
func ReadGlossary(path string) (*Glossary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g Glossary
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &g, nil
}

// Relevant returns the glossary entries whose English term appears in at least
// one of the given strings. Sending only the terms a batch can use keeps the
// prompt small and the model's attention on them.
func (g *Glossary) Relevant(texts []string) map[string]string {
	if g == nil || len(g.Terms) == 0 {
		return nil
	}
	joined := strings.ToLower(strings.Join(texts, "\n"))
	out := map[string]string{}
	for en, ko := range g.Terms {
		if strings.Contains(joined, strings.ToLower(en)) {
			out[en] = ko
		}
	}
	return out
}

// SystemPrompt is the standing instruction. It is deliberately identical
// across batches so it caches, and it says nothing about specific strings.
const SystemPrompt = `You translate UI and flavour text for a Europa Universalis V mod called The Idea Variation 2, from English into Korean.

The mod adds Idea Groups and National Ideas, a system Europa Universalis IV had and EU5 dropped. Its text is grand-strategy UI: idea names, tooltips, modifier descriptions, event messages, and short flavour paragraphs about statecraft, warfare, trade and religion in the 1337-1837 period.

RULES

1. Placeholders. Some strings contain placeholders written as ` + "⟦T1⟧, ⟦T2⟧ … and ⟦N1⟧, ⟦N2⟧ …" + `. These stand for values the engine substitutes at runtime: a T placeholder is markup or a cross-reference, an N placeholder is a number.
   - Reproduce every placeholder from the source exactly, character for character.
   - Never add a placeholder that is not in the source, never drop one, never renumber one, never translate the letter inside one.
   - You MAY move them wherever Korean word order requires. That is why they are numbered.
   - A placeholder often stands for the noun the sentence is about, so the sentence must read naturally with it in place.

2. Korean particles. The value a placeholder resolves to is unknown at translation time, so its final consonant is unknown too. When a particle attaches directly to a placeholder, use the paired form the game's own Korean uses: 을(를), 이(가), 은(는), 와(과), 으로(로). Between ordinary words use the correct single particle as normal.

3. Register. Use the tone of a strategy game interface: plain, declarative, reasonably formal. Prefer the 합니다 style for sentences and bare noun phrases for labels and titles. No casual speech.

4. Keep it tight. UI text sits in fixed-width panels. Prefer the shorter natural phrasing over the more literal one.

5. Names and terms. Use established Korean forms for country, culture and religion names. Follow the supplied glossary exactly where it applies.

6. Leave untranslated anything that is plainly an internal identifier rather than prose. For a bare "OK" button label use 확인.

OUTPUT

Return a single JSON object mapping each id to its Korean translation, and nothing else:

{"1": "번역문", "2": "번역문"}

Every id you were given must appear exactly once. No commentary, no explanation, no markdown fences.`

// BuildUserMessage renders one batch into the request body.
func BuildUserMessage(items []Item, glossary map[string]string) string {
	var b strings.Builder

	if len(glossary) > 0 {
		terms := make([]string, 0, len(glossary))
		for en := range glossary {
			terms = append(terms, en)
		}
		sort.Strings(terms)
		b.WriteString("GLOSSARY - use these renderings exactly:\n")
		for _, en := range terms {
			fmt.Fprintf(&b, "  %s -> %s\n", en, glossary[en])
		}
		b.WriteString("\n")
	}

	b.WriteString("Translate each of these into Korean.\n\n")
	payload := make(map[string]string, len(items))
	for _, it := range items {
		payload[it.ID] = it.Source
	}
	enc, _ := json.MarshalIndent(payload, "", "  ")
	b.Write(enc)
	return b.String()
}
