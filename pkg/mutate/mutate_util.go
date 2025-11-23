package mutate

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"unicode"

	"github.com/hashcracky/brainstorm/pkg/structs"
)

// indexedLine represents a single input line tagged with its sequence index.
//
// Args:
// index: int - Sequential index of the line in the overall input.
// data: []byte - Raw line bytes without trailing newline.
//
// Returns:
// indexedLine - Indexed representation of a line.
type indexedLine struct {
	index int
	data  []byte
}

// indexedResult represents a transformed output line tagged with its sequence index.
//
// Args:
// index: int - Sequential index of the line in the overall input.
// data: []byte - Transformed line bytes. Nil or empty means no output for that line.
//
// Returns:
// indexedResult - Indexed representation of a transformation result.
type indexedResult struct {
	index int
	data  []byte
}

// ProcessStream reads from stdin, applies transformations using a worker pool,
// and writes results to stdout while preserving input order.
//
// The worker count is derived automatically from GOMAXPROCS and cannot be
// customized via command-line flags.
//
// Args:
// cfg: *structs.Config - Application configuration.
//
// Returns:
// error - Any error encountered during processing.
func ProcessStream(cfg *structs.Config) error {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat stdin: %w", err)
	}

	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return fmt.Errorf("no stdin detected; supply input via a pipe or redirection")
	}

	workerCount := runtime.GOMAXPROCS(0)
	if workerCount < 1 {
		workerCount = 1
	}

	reader := bufio.NewReaderSize(os.Stdin, 1<<20)
	writer := bufio.NewWriterSize(os.Stdout, 1<<20)

	defer func() {
		_ = writer.Flush()
	}()

	inputCh := make(chan indexedLine, workerCount*2)
	resultCh := make(chan indexedResult, workerCount*2)

	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for line := range inputCh {
				processed := TransformLine(cfg, line.data)
				resultCh <- indexedResult{
					index: line.index,
					data:  processed,
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	readIndex := 0

	for {
		rawLine, readErr := reader.ReadBytes('\n')

		if len(rawLine) > 0 {
			hasNewline := rawLine[len(rawLine)-1] == '\n'

			var raw []byte

			if hasNewline {
				raw = rawLine[:len(rawLine)-1]
			} else {
				raw = rawLine
			}

			lineCopy := make([]byte, len(raw))
			copy(lineCopy, raw)

			inputCh <- indexedLine{
				index: readIndex,
				data:  lineCopy,
			}

			readIndex++

			if readErr != nil {
				if readErr.Error() == "EOF" {
					break
				}

				close(inputCh)

				return fmt.Errorf("error reading from stdin: %w", readErr)
			}
		} else {
			if readErr != nil {
				if readErr.Error() == "EOF" {
					break
				}

				close(inputCh)

				return fmt.Errorf("error reading from stdin: %w", readErr)
			}
		}
	}

	close(inputCh)

	nextIndexToWrite := 0
	pending := make(map[int]indexedResult)

	for res := range resultCh {
		pending[res.index] = res

		for {
			nextRes, exists := pending[nextIndexToWrite]
			if !exists {
				break
			}

			delete(pending, nextIndexToWrite)

			if len(nextRes.data) > 0 {
				if _, werr := writer.Write(nextRes.data); werr != nil {
					return fmt.Errorf("failed to write output: %w", werr)
				}

				if _, werr := writer.Write([]byte{'\n'}); werr != nil {
					return fmt.Errorf("failed to write newline: %w", werr)
				}
			}

			nextIndexToWrite++
		}
	}

	return nil
}

// filterLines checks each line and skips those that consist only of digits or
// special characters and those that are unlikely to contain words.
//
// Args:
// data ([]byte): The byte slice containing the data to be processed.
//
// Returns:
// []byte: The processed byte slice with filtered lines.
func filterLines(data []byte) []byte {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var result strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			continue
		}

		if isAllDigitsOrSpecialChars(line) {
			continue
		}

		if !likelyContainsWords(line) {
			continue
		}

		result.WriteString(line + "\n")
	}

	return []byte(result.String())
}

// isAllDigitsOrSpecialChars checks if a string contains only digits or special
// characters.
//
// Args:
// s (string): The string to check.
//
// Returns:
// bool: True if the string contains only digits or special characters, false
// otherwise.
func isAllDigitsOrSpecialChars(s string) bool {
	hasLetter := false

	for _, char := range s {
		if unicode.IsLetter(char) {
			hasLetter = true
			break
		}
	}

	return !hasLetter
}

// likelyContainsWords checks a string to see if there are atleast 5 characters
// in a row that are not digits or special characters and ensures that there is
// atleast one vowel in the string.
//
// Args:
// s (string): The string to check.
//
// Returns:
// bool: True if the string likely contains words, false otherwise.
func likelyContainsWords(s string) bool {
	if len(s) < 5 {
		return false
	}

	vowelCount := 0

	for i := 0; i < len(s)-4; i++ {
		if isWordLike(s[i : i+5]) {
			vowelCount++
		}
	}

	return vowelCount > 0
}

// isWordLike checks if a substring contains at least one vowel and no more
// than one digit or special character.
//
// Args:
// s (string): The substring to check.
//
// Returns:
// bool: True if the substring is likely a word, false otherwise.
func isWordLike(s string) bool {
	vowels := "aeiouAEIOU"
	digitOrSpecialCount := 0
	hasVowel := false

	for _, char := range s {
		if strings.ContainsRune(vowels, char) {
			hasVowel = true
		} else if !unicode.IsLetter(char) {
			digitOrSpecialCount++
		}
	}

	return hasVowel && digitOrSpecialCount <= 1
}

// removeTrailingNonLettersDigits removes trailing characters from each line
// that are not letters.
//
// Args:
// data ([]byte): The byte slice containing the data to be processed.
//
// Returns:
// []byte: The processed byte slice with trailing non-letter characters removed.
func removeTrailingNonLettersDigits(data []byte) []byte {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var result strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		processedLine := strings.TrimRightFunc(line, func(r rune) bool {
			return !unicode.IsLetter(r)
		})

		result.WriteString(processedLine + "\n")
	}

	return []byte(result.String())
}

// removeLeadingNonLettersDigits removes leading characters from each line
// that are not letters.
//
// Args:
// data ([]byte): The byte slice containing the data to be processed.
//
// Returns:
// []byte: The processed byte slice with leading non-letter characters removed.
func removeLeadingNonLettersDigits(data []byte) []byte {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var result strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		processedLine := strings.TrimLeftFunc(line, func(r rune) bool {
			return !unicode.IsLetter(r)
		})

		result.WriteString(processedLine + "\n")
	}

	return []byte(result.String())
}

// enforceLengthRange filters the input byte slice to only include strings
// between minLength and maxLength characters inclusive.
//
// Args:
// input ([]byte): The input byte slice to filter.
// minLength (int): The minimum length of strings to include.
// maxLength (int): The maximum length of strings to include.
//
// Returns:
// []byte: A new byte slice with strings within the specified length range.
func enforceLengthRange(input []byte, minLength int, maxLength int) []byte {
	lines := strings.Split(string(input), "\n")
	var filtered []string

	for _, line := range lines {
		if len(line) >= minLength && len(line) <= maxLength {
			filtered = append(filtered, line)
		}
	}

	return []byte(strings.Join(filtered, "\n"))
}
