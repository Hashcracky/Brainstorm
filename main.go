// Package main controls the user interaction logic for the brainstorm application.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashcracky/brainstorm/pkg/mutate"
	"github.com/hashcracky/brainstorm/pkg/structs"
)

// version is the current version of the brainstorm application.
var version = "0.1.3"

// parseRangeFlag parses a range in the form "start-end" and returns
// the corresponding integer bounds.
//
// Args:
// value: string - Raw range string (for example, "1-5").
// defaultStart: int - Default start value if parsing fails or is empty.
// defaultEnd: int - Default end value if parsing fails or is empty.
//
// Returns:
// int - Parsed start value.
// int - Parsed end value.
// error - Error if the value is malformed.
func parseRangeFlag(value string, defaultStart int, defaultEnd int) (int, int, error) {
	if strings.TrimSpace(value) == "" {
		return defaultStart, defaultEnd, nil
	}

	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format %q, expected start-end", value)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range start %q: %w", parts[0], err)
	}

	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range end %q: %w", parts[1], err)
	}

	if start <= 0 || end <= 0 {
		return 0, 0, fmt.Errorf("range values must be positive: %q", value)
	}

	if start > end {
		return 0, 0, fmt.Errorf("range start must be <= end: %q", value)
	}

	return start, end, nil
}

// parseFlags parses command-line flags and returns a Config.
//
// The supported flags are:
//
//	-w: string - N-gram word length range, in the form start-end (for example, 1-5).
//	-l: string - Final output length range, in the form min-max (for example, 4-32).
//	-unicode: bool - Relax Latin-centric heuristics to include non-Latin multi-byte letter sequences.
//
// Returns:
// *structs.Config - Pointer to the populated configuration struct.
func parseFlags() *structs.Config {
	nGramRange := flag.String(
		"w",
		"1-5",
		"N-gram word length range in the form start-end (for example, 1-5).",
	)

	outLenRange := flag.String(
		"l",
		"4-32",
		"Final output length range in the form min-max (for example, 4-32).",
	)

	includeNonLatin := flag.Bool(
		"unicode",
		false,
		"Include non-Latin multi-byte letter sequences by relaxing Latin vowel heuristics.",
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of Brainstorm version (%s):\n\n", version)
		fmt.Fprintf(os.Stderr, "input | brainstorm [options] > output\n\n")
		fmt.Fprintf(os.Stderr, "Accepts standard input and writes transformed output to standard output.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	nStart, nEnd, nErr := parseRangeFlag(*nGramRange, 1, 5)
	if nErr != nil {
		fmt.Fprintf(os.Stderr, "[!] Invalid -w value: %v\n", nErr)
		os.Exit(1)
	}

	outStart, outEnd, outErr := parseRangeFlag(*outLenRange, 4, 32)
	if outErr != nil {
		fmt.Fprintf(os.Stderr, "[!] Invalid -l value: %v\n", outErr)
		os.Exit(1)
	}

	cfg := &structs.Config{
		NGramMin:        nStart,
		NGramMax:        nEnd,
		OutMinLength:    outStart,
		OutMaxLength:    outEnd,
		IncludeNonLatin: *includeNonLatin,
	}

	return cfg
}

// main is the entry point for the brainstorm application.
func main() {
	cfg := parseFlags()

	if err := mutate.ProcessStream(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[!] %s.\n", err)
		os.Exit(1)
	}
}
