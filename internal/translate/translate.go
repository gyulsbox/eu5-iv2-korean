package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/build"
	"github.com/gyulsbox/EUV_Idea_Variation_2_kr/internal/inventory"
)

// DefaultModel is Claude Haiku 4.5. The job is bounded, highly repetitive UI
// text with a glossary and a hard validation gate behind it, which is the
// shape Haiku handles well and cheaply.
const DefaultModel = "claude-haiku-4-5"

// Item is one string handed to the model.
type Item struct {
	// ID is a short per-batch handle. Templates are too long to use as keys
	// in the response, and a short id keeps output tokens down.
	ID     string
	Source string
	// Index is the item's position in the catalog.
	Index int
}

// Options configures a translation run.
type Options struct {
	Model     string
	Glossary  *Glossary
	BatchSize int
	// Concurrency is how many batches are in flight at once.
	Concurrency int
	MaxRetries  int
	// Limit caps how many units are attempted; 0 means all of them.
	Limit int
	// Retranslate redoes units that already carry a translation.
	Retranslate bool
	// Progress is called after each batch settles.
	Progress func(done, total, ok, rejected int)
}

func (o *Options) applyDefaults() {
	if o.Model == "" {
		o.Model = DefaultModel
	}
	if o.BatchSize <= 0 {
		o.BatchSize = 40
	}
	if o.Concurrency <= 0 {
		o.Concurrency = 4
	}
	if o.MaxRetries <= 0 {
		o.MaxRetries = 5
	}
}

// Stats reports what a run achieved.
type Stats struct {
	Attempted int
	Accepted  int
	// Rejected counts translations that came back with damaged placeholders.
	// They are discarded rather than written, so the unit stays untranslated
	// and the build keeps English for it.
	Rejected  int
	Missing   int
	Batches   int
	Retries   int
	InTokens  int64
	OutTokens int64
	// Rejections samples what went wrong, for tuning the prompt.
	Rejections []Rejection
}

// Rejection records one discarded translation.
type Rejection struct {
	Rep    string
	Source string
	Got    string
	Reason string
}

// Cost estimates the run's spend in USD at Haiku 4.5 list prices.
func (s Stats) Cost() float64 {
	const inPerM, outPerM = 1.00, 5.00
	return float64(s.InTokens)/1e6*inPerM + float64(s.OutTokens)/1e6*outPerM
}

// Pending returns the catalog units that still need translating: prose only,
// since reference-only and empty units have no text to translate.
func Pending(cat *build.Catalog, retranslate bool) []Item {
	var out []Item
	for i, u := range cat.Units {
		if u.Class != string(inventory.ClassProse) {
			continue
		}
		if u.Korean != "" && !retranslate {
			continue
		}
		out = append(out, Item{Source: u.Template, Index: i})
	}
	return out
}

// Run translates the catalog in place. It only ever writes a unit's Korean
// field after the translation has passed placeholder validation, so a failed
// call or a mangled response leaves the catalog no worse than it found it.
func Run(ctx context.Context, client anthropic.Client, cat *build.Catalog, o Options) (*Stats, error) {
	o.applyDefaults()

	items := Pending(cat, o.Retranslate)
	if o.Limit > 0 && len(items) > o.Limit {
		items = items[:o.Limit]
	}
	stats := &Stats{Attempted: len(items)}
	if len(items) == 0 {
		return stats, nil
	}

	batches := chunk(items, o.BatchSize)
	stats.Batches = len(batches)

	var (
		mu   sync.Mutex
		done int
		wg   sync.WaitGroup
		sem  = make(chan struct{}, o.Concurrency)
		errs []error
	)

	for _, batch := range batches {
		wg.Add(1)
		go func(batch []Item) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := translateBatch(ctx, client, batch, o)

			mu.Lock()
			defer mu.Unlock()
			done++
			if err != nil {
				errs = append(errs, err)
			} else {
				stats.Retries += res.retries
				stats.InTokens += res.inTokens
				stats.OutTokens += res.outTokens
				for _, it := range batch {
					ko, got := res.translations[it.ID]
					if !got {
						stats.Missing++
						continue
					}
					if reason := checkPlaceholders(it.Source, ko); reason != "" {
						stats.Rejected++
						if len(stats.Rejections) < 20 {
							stats.Rejections = append(stats.Rejections, Rejection{
								Rep:    cat.Units[it.Index].Rep,
								Source: it.Source,
								Got:    ko,
								Reason: reason,
							})
						}
						continue
					}
					cat.Units[it.Index].Korean = ko
					stats.Accepted++
				}
			}
			if o.Progress != nil {
				o.Progress(done, len(batches), stats.Accepted, stats.Rejected)
			}
		}(batch)
	}
	wg.Wait()

	if len(errs) > 0 {
		return stats, fmt.Errorf("%d batch(es) failed, first: %w", len(errs), errs[0])
	}
	return stats, nil
}

type batchResult struct {
	translations map[string]string
	retries      int
	inTokens     int64
	outTokens    int64
}

func translateBatch(ctx context.Context, client anthropic.Client, batch []Item, o Options) (*batchResult, error) {
	texts := make([]string, len(batch))
	for i := range batch {
		batch[i].ID = fmt.Sprint(i + 1)
		texts[i] = batch[i].Source
	}
	user := BuildUserMessage(batch, o.Glossary.Relevant(texts))

	res := &batchResult{}
	var lastErr error

	for attempt := 0; attempt <= o.MaxRetries; attempt++ {
		if attempt > 0 {
			res.retries++
			if err := sleep(ctx, backoff(attempt)); err != nil {
				return nil, err
			}
		}

		msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(o.Model),
			MaxTokens: 16000,
			System: []anthropic.TextBlockParam{{
				Text: SystemPrompt,
				// The system prompt is identical for every batch, so caching
				// it turns most of the input cost into cache reads.
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			}},
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
			},
		})
		if err != nil {
			if retryable(err) {
				lastErr = err
				continue
			}
			return nil, err
		}

		res.inTokens += msg.Usage.InputTokens + msg.Usage.CacheReadInputTokens + msg.Usage.CacheCreationInputTokens
		res.outTokens += msg.Usage.OutputTokens

		var text strings.Builder
		for _, block := range msg.Content {
			if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
				text.WriteString(tb.Text)
			}
		}

		parsed, err := parseResponse(text.String())
		if err != nil {
			// A malformed response is worth one more try; the model usually
			// gets the JSON right on a second pass.
			lastErr = err
			continue
		}
		res.translations = parsed
		return res, nil
	}
	return nil, fmt.Errorf("batch failed after %d attempts: %w", o.MaxRetries+1, lastErr)
}

// parseResponse extracts the JSON object from a response, tolerating the
// markdown fences and stray prose that models occasionally add.
func parseResponse(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		if j := strings.IndexByte(s, '\n'); j >= 0 {
			s = s[j+1:]
		}
		if j := strings.Index(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object in response")
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(s[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("parsing response JSON: %w", err)
	}
	return out, nil
}

// checkPlaceholders enforces the one invariant the model can break silently.
// It returns an empty string when the translation is sound.
//
// This is the same multiset rule the validator applies to finished strings,
// but applied here so a damaged translation is never written to the catalog
// in the first place.
func checkPlaceholders(source, translation string) string {
	want := countEach(inventory.Placeholders(source))
	got := countEach(inventory.Placeholders(translation))

	var lost, added []string
	for p, n := range want {
		if got[p] < n {
			lost = append(lost, p)
		}
	}
	for p, n := range got {
		if want[p] < n {
			added = append(added, p)
		}
	}
	switch {
	case len(lost) > 0 && len(added) > 0:
		return "lost " + strings.Join(lost, " ") + ", added " + strings.Join(added, " ")
	case len(lost) > 0:
		return "lost " + strings.Join(lost, " ")
	case len(added) > 0:
		return "added " + strings.Join(added, " ")
	}
	if strings.TrimSpace(translation) == "" && strings.TrimSpace(source) != "" {
		return "empty translation"
	}
	return ""
}

func countEach(items []string) map[string]int {
	m := map[string]int{}
	for _, s := range items {
		m[s]++
	}
	return m
}

func chunk(items []Item, size int) [][]Item {
	var out [][]Item
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		// Copy so each batch can carry its own ids.
		batch := make([]Item, end-i)
		copy(batch, items[i:end])
		out = append(out, batch)
	}
	return out
}

// retryable reports whether an error is worth another attempt: rate limits,
// server errors, and connection failures.
func retryable(err error) bool {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 429, apiErr.StatusCode == 408, apiErr.StatusCode == 409:
			return true
		case apiErr.StatusCode >= 500:
			return true
		default:
			return false
		}
	}
	// Connection-level failures carry no status code.
	return true
}

// backoff returns an exponential delay with jitter, capped at a minute.
func backoff(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt)) * time.Second
	if d > time.Minute {
		d = time.Minute
	}
	return d + time.Duration(rand.Int63n(int64(time.Second)))
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
