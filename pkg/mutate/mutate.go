// Package mutate contains logic for parsing strings
package mutate

import (
	"bufio"
	"strings"

	"github.com/hashcracky/brainstorm/pkg/structs"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TransformLine applies the core brainstorm transformation to a single input line.
//
// Args:
// cfg: *structs.Config - Application configuration.
// line: []byte - Raw input line (without trailing newline).
//
// Returns:
// []byte - Transformed line (without trailing newline).
func TransformLine(cfg *structs.Config, line []byte) []byte {
	line = removeTrailingNonLettersDigits(line)
	line = removeLeadingNonLettersDigits(line)
	line = filterLines(cfg, line)

	if len(line) == 0 {
		return nil
	}

	processedChunk := generateNGramSliceBytes(line, cfg.NGramMin, cfg.NGramMax)
	processedChunk = []byte(strings.Join(prepareStringForTransformations(processedChunk), "\n"))

	processedChunk = []byte(strings.Join(applyPostFilters(processedChunk), "\n"))

	return enforceLengthRange(processedChunk, cfg.OutMinLength, cfg.OutMaxLength)
}

// generateNGramSliceBytes takes a byte slice and generates a new byte slice
// using the GenerateNGramsBytes function and combines the results.
//
// Args:
// input ([]byte): The original byte slice to generate n-grams from
// wordRangeStart (int): The starting number of words to use for n-grams
// wordRangeEnd (int): The ending iteration number of words to use for n-grams
//
// Returns:
// []byte: A new byte slice with the n-grams generated.
func generateNGramSliceBytes(input []byte, wordRangeStart int, wordRangeEnd int) []byte {
	data := string(input)
	lines := strings.Split(data, "\n")
	var newList []string

	for _, line := range lines {
		nGrams := generateNGrams(line, wordRangeStart, wordRangeEnd)
		newList = append(newList, nGrams...)
	}

	return []byte(strings.Join(newList, "\n"))
}

// generateNGrams generates n-grams from a string of text and returns a slice of n-grams.
//
// Args:
// text (string): The text to generate n-grams from.
// wordRangeStart (int): The starting number of words to use for n-grams.
// wordRangeEnd (int): The ending iteration number of words to use for n-grams.
//
// Returns:
// []string: A slice of n-grams.
func generateNGrams(text string, wordRangeStart int, wordRangeEnd int) []string {
	words := strings.Fields(text)
	var nGrams []string

	for i := wordRangeStart; i <= wordRangeEnd; i++ {
		if i <= 0 || i > len(words) {
			continue
		}

		for j := 0; j <= len(words)-i; j++ {
			nGram := strings.Join(words[j:j+i], " ")
			nGram = strings.TrimSpace(nGram)
			nGram = strings.TrimLeft(nGram, " ")
			nGram = strings.ReplaceAll(nGram, ".", "")
			nGram = strings.ReplaceAll(nGram, ",", "")
			nGram = strings.ReplaceAll(nGram, ";", "")
			nGrams = append(nGrams, nGram)
		}
	}

	return nGrams
}

// prepareStringForTransformations processes each line in the input byte slice,
// removes unwanted characters, normalizes each line, and generates various
// transformed versions for each line.
//
// Args:
// data ([]byte): The byte slice containing lines to process.
//
// Returns:
// []string: A flattened slice of all prepared string variants for all lines.
func prepareStringForTransformations(data []byte) []string {
	input := string(data)
	scanner := bufio.NewScanner(strings.NewReader(input))

	var results []string

	for scanner.Scan() {
		line := scanner.Text()

		clean := strings.ReplaceAll(line, "\x00", "")
		clean = strings.ReplaceAll(clean, "\n", "")
		clean = strings.ReplaceAll(clean, "\t", "")
		clean = strings.ReplaceAll(clean, "\r", "")
		clean = strings.ReplaceAll(clean, "\f", "")
		clean = strings.ReplaceAll(clean, "\v", "")

		if strings.TrimSpace(clean) == "" {
			continue
		}

		if strings.Contains(clean, " ") {
			results = append(
				results,
				strings.ReplaceAll(
					cases.Title(language.Und, cases.NoLower).String(clean),
					" ",
					"",
				),
			)
		} else {
			results = append(results, strings.ReplaceAll(clean, " ", ""))
		}
	}

	return results
}

// applyPostFilters applies post-processing filters on the transformed output
// lines, including removing unbalanced leading-quote or leading-bracket
// variants and adding apostrophe-stripped variants.
//
// Args:
// data ([]byte): The byte slice containing transformed lines.
//
// Returns:
// []string: A slice of filtered and augmented lines.
func applyPostFilters(data []byte) []string {
	input := string(data)
	scanner := bufio.NewScanner(strings.NewReader(input))

	var filtered []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		if hasUnbalancedLeadingDelimiter(line) {
			continue
		}

		filtered = append(filtered, line)

		apostropheFreeVariants := generateApostropheFreeVariants(line)
		filtered = append(filtered, apostropheFreeVariants...)
	}

	return filtered
}

// hasUnbalancedLeadingDelimiter checks whether a string starts with an opening
// quote or bracket and lacks the corresponding closing quote or bracket later
// in the token.
//
// Args:
// s (string): The string to inspect.
//
// Returns:
// bool: True if the string has an unbalanced leading delimiter.
func hasUnbalancedLeadingDelimiter(s string) bool {
	if s == "" {
		return false
	}

	runes := []rune(s)

	openToClose := map[rune][]rune{
		'(': {')'},
		'[': {']'},
		'{': {'}'},
		'<': {'>'},
		'"': {'"', '”'},
		'“': {'”', '"'},
		'‘': {'’', '\''},
	}

	first := runes[0]

	allowedClosers, isOpening := openToClose[first]
	if !isOpening {
		return false
	}

	for _, r := range runes[1:] {
		for _, c := range allowedClosers {
			if r == c {
				return false
			}
		}
	}

	return true
}

// generateApostropheFreeVariants returns variants of the input string where
// apostrophes are removed. The original string is not included in the
// returned slice.
//
// Args:
// s (string): The string to generate variants from.
//
// Returns:
// []string: A slice of apostrophe-free variants, or an empty slice if no
// apostrophes are present.
func generateApostropheFreeVariants(s string) []string {
	if !strings.ContainsAny(s, "'’") {
		return nil
	}

	variant := strings.Map(
		func(r rune) rune {
			if r == '\'' || r == '’' {
				return -1
			}

			return r
		},
		s,
	)

	if variant == "" || variant == s {
		return nil
	}

	return []string{variant}
}
