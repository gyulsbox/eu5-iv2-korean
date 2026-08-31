package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/gyulsbox/eu5-iv2-korean/internal/build"
	"github.com/gyulsbox/eu5-iv2-korean/internal/translate"
)

type translateOpts struct {
	catalog     string
	glossary    string
	model       string
	batch       int
	concurrency int
	limit       int
	retries     int
	retranslate bool
	dryRun      bool
}

func registerTranslateFlags(fs *flag.FlagSet, o *translateOpts) {
	fs.StringVar(&o.catalog, "catalog", "", "catalog to fill in, written back in place")
	fs.StringVar(&o.glossary, "glossary", "", "glossary of fixed term renderings")
	fs.StringVar(&o.model, "model", translate.DefaultModel, "model to translate with")
	fs.IntVar(&o.batch, "batch", 40, "strings per request")
	fs.IntVar(&o.concurrency, "concurrency", 4, "requests in flight at once")
	fs.IntVar(&o.limit, "limit", 0, "translate at most this many units (0 = all)")
	fs.IntVar(&o.retries, "retries", 5, "retries per batch on rate limits and server errors")
	fs.BoolVar(&o.retranslate, "retranslate", false, "redo units that already have a translation")
	fs.BoolVar(&o.dryRun, "dry-run", false, "report what would be sent, without calling the API")
}

func runTranslate(args []string) error {
	var o translateOpts
	fs := flag.NewFlagSet("translate", flag.ContinueOnError)
	registerTranslateFlags(fs, &o)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if o.catalog == "" {
		return fmt.Errorf("--catalog is required")
	}

	cat, err := build.ReadCatalog(o.catalog)
	if err != nil {
		return err
	}

	var glossary *translate.Glossary
	if o.glossary != "" {
		glossary, err = translate.ReadGlossary(o.glossary)
		if err != nil {
			return err
		}
	}

	opts := translate.Options{
		Model:       o.model,
		Glossary:    glossary,
		BatchSize:   o.batch,
		Concurrency: o.concurrency,
		MaxRetries:  o.retries,
		Limit:       o.limit,
		Retranslate: o.retranslate,
	}

	pending := translate.Pending(cat, o.retranslate)
	if o.limit > 0 && len(pending) > o.limit {
		pending = pending[:o.limit]
	}

	fmt.Printf("═══ translate ═══\n")
	fmt.Printf("  catalog     %s\n", o.catalog)
	fmt.Printf("  model       %s\n", o.model)
	if glossary != nil {
		fmt.Printf("  glossary    %s (%d terms)\n", o.glossary, len(glossary.Terms))
	} else {
		fmt.Printf("  glossary    (none)\n")
	}
	fmt.Printf("  units       %d of %d translated, %d to do\n",
		cat.Translated(), len(cat.Units), len(pending))

	if len(pending) == 0 {
		fmt.Printf("\n  nothing to do\n")
		return nil
	}

	var chars int
	for _, it := range pending {
		chars += len([]rune(it.Source))
	}
	batches := (len(pending) + o.batch - 1) / o.batch
	fmt.Printf("  work        %d chars in %d batches of %d, %d at a time\n",
		chars, batches, o.batch, o.concurrency)

	if o.dryRun {
		fmt.Printf("\n  dry run - nothing sent. Sample of what a batch looks like:\n\n")
		sample := pending
		if len(sample) > 5 {
			sample = sample[:5]
		}
		for i := range sample {
			sample[i].ID = fmt.Sprint(i + 1)
		}
		texts := make([]string, len(sample))
		for i, s := range sample {
			texts[i] = s.Source
		}
		fmt.Println(translate.BuildUserMessage(sample, glossary.Relevant(texts)))
		return nil
	}

	// The SDK resolves credentials itself: ANTHROPIC_API_KEY, then
	// ANTHROPIC_AUTH_TOKEN, then a profile stored by `ant auth login`.
	client := anthropic.NewClient()

	start := time.Now()
	opts.Progress = func(done, total, ok, rejected int) {
		fmt.Printf("\r  batch %d/%d  accepted %d  rejected %d  %s elapsed   ",
			done, total, ok, rejected, time.Since(start).Round(time.Second))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stats, runErr := translate.Run(ctx, client, cat, opts)
	fmt.Printf("\n")

	// Save whatever succeeded even when some batches failed, so a partial run
	// is never wasted; rerunning picks up only what is still missing.
	cat.Sort()
	if err := build.WriteCatalog(o.catalog, cat); err != nil {
		return err
	}

	fmt.Printf("\n═══ result ═══\n")
	fmt.Printf("  attempted        %8d\n", stats.Attempted)
	fmt.Printf("  accepted         %8d\n", stats.Accepted)
	fmt.Printf("  rejected         %8d   (placeholders damaged; left untranslated)\n", stats.Rejected)
	if stats.Missing > 0 {
		fmt.Printf("  missing          %8d   (model omitted these ids)\n", stats.Missing)
	}
	if stats.Retries > 0 {
		fmt.Printf("  retries          %8d\n", stats.Retries)
	}
	fmt.Printf("  tokens           %8d in, %d out\n", stats.InTokens, stats.OutTokens)
	fmt.Printf("  approx cost      $%.2f\n", stats.Cost())
	fmt.Printf("  catalog is now   %d of %d units translated\n", cat.Translated(), len(cat.Units))

	if len(stats.Rejections) > 0 {
		fmt.Printf("\n  rejected samples:\n")
		for _, r := range stats.Rejections {
			fmt.Printf("    %s (%s)\n      en: %s\n      ko: %s\n",
				r.Rep, r.Reason, trunc(oneLine(r.Source), 90), trunc(oneLine(r.Got), 90))
		}
	}

	if runErr != nil {
		return runErr
	}
	fmt.Printf("\n  next: iv2loc build --src <IV2> --out <pack> --catalog %s\n", o.catalog)
	return nil
}
