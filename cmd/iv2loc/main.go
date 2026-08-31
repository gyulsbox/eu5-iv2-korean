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
	case "validate":
		err = runValidate(args)
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

  iv2loc validate --src <IV2 mod path> [--out <pack>] [--baseline <tree>]...
        Check that every translated string keeps its markup intact and that
        the pack redefines nothing the base game or another mod owns.

extract flags:
`)
	ef := flag.NewFlagSet("extract", flag.ContinueOnError)
	registerExtractFlags(ef, &extractOpts{})
	ef.SetOutput(os.Stderr)
	ef.PrintDefaults()

	fmt.Fprint(os.Stderr, "\nvalidate flags:\n")
	vf := flag.NewFlagSet("validate", flag.ContinueOnError)
	registerValidateFlags(vf, &validateOpts{})
	vf.SetOutput(os.Stderr)
	vf.PrintDefaults()
}
