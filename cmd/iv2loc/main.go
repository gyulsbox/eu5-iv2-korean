// Command iv2loc builds and maintains a Korean localization pack for the
// Europa Universalis V mod "The Idea Variation 2".
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "extract":
		err = runExtract(args)
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "iv2loc: unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "iv2loc %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `iv2loc - EU5 "The Idea Variation 2" Korean localization toolchain

usage:
  iv2loc extract --src <IV2 mod path> [--json inventory.json]
        Parse the mod's source localization and report how many keys exist
        and how many distinct strings actually have to be translated.

flags:
`)
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	registerExtractFlags(fs, &extractOpts{})
	fs.SetOutput(os.Stderr)
	fs.PrintDefaults()
}
