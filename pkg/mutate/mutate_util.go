package mutate

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"unicode"

	"github.com/hashcracky/brainstorm/pkg/structs"
)

// ProcessStream reads from stdin, processes lines concurrently without preserving
// order, and writes results to stdout as soon as they are available.
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

	reader := bufio.NewReaderSize(os.Stdin, 1<<20)
	writer := bufio.NewWriterSize(os.Stdout, 1<<20)

	var writeMu sync.Mutex

	defer func() {
		writeMu.Lock()
		_ = writer.Flush()
		writeMu.Unlock()
	}()

	type lineTask struct {
		Data []byte
	}

	taskCh := make(chan lineTask, 1024)

	workerCount := runtime.NumCPU()
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for task := range taskCh {
				lineCopy := make([]byte, len(task.Data))
				copy(lineCopy, task.Data)

				processed := TransformLine(cfg, lineCopy)

				if len(processed) == 0 {
					continue
				}

				writeMu.Lock()

				_, werr := writer.Write(processed)
				if werr == nil {
					_, werr = writer.Write([]byte{'\n'})
				}

				writeMu.Unlock()

				if werr != nil {
					return
				}
			}
		}()
	}

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

			taskCh <- lineTask{
				Data: raw,
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}

			close(taskCh)
			wg.Wait()

			return fmt.Errorf("error reading from stdin: %w", readErr)
		}
	}

	close(taskCh)
	wg.Wait()

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

	if !looksLikeWordPattern(s) {
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

// isVowel returns whether a rune is a vowel.
//
// Args:
// r: rune - Character to test.
//
// Returns:
// bool - True if the rune is a vowel, false otherwise.
func isVowel(r rune) bool {
	vowels := "aeiouAEIOU"

	return strings.ContainsRune(vowels, r)
}

// isLetterLike returns whether a rune should be treated as a word letter.
//
// Args:
// r: rune - Character to test.
//
// Returns:
// bool - True if the rune is letter-like, false otherwise.
func isLetterLike(r rune) bool {
	return unicode.IsLetter(r) || r == '\''
}

// looksLikeWordPattern applies heuristic checks for vowel density and
// consonant run length to decide whether a string looks like a word.
//
// Args:
// s: string - Input string.
//
// Returns:
// bool - True if the string passes heuristic word checks, false otherwise.
func looksLikeWordPattern(s string) bool {
	if len(s) == 0 {
		return false
	}

	if hasSuspiciousAlphaNumericRuns(s) {
		return false
	}

	if hasHighNonLetterDensity(s) {
		return false
	}

	if containsTooManyUncommonClusters(s) {
		return false
	}

	syllables := countSyllableLikeSegments(s)

	if syllables == 0 {
		return false
	}

	if len(s) >= 8 && syllables < 2 {
		return false
	}

	var (
		vowelCount          int
		letterCount         int
		maxConsonantRun     int
		currentConsonantRun int
		maxVowelRun         int
		currentVowelRun     int
	)

	for _, r := range s {
		if !isLetterLike(r) {
			continue
		}

		letterCount++

		if isVowel(r) {
			vowelCount++
			currentVowelRun++
			if currentVowelRun > maxVowelRun {
				maxVowelRun = currentVowelRun
			}
			currentConsonantRun = 0
		} else {
			currentConsonantRun++
			if currentConsonantRun > maxConsonantRun {
				maxConsonantRun = currentConsonantRun
			}
			currentVowelRun = 0
		}
	}

	if letterCount == 0 {
		return false
	}

	vowelRatio := float64(vowelCount) / float64(letterCount)

	if vowelRatio < 0.25 {
		return false
	}

	if vowelRatio > 0.8 {
		return false
	}

	if maxConsonantRun > 4 {
		return false
	}

	if maxVowelRun > 3 {
		return false
	}

	return true
}

// containsTooManyUncommonClusters checks for a high ratio of uncommon
// letter clusters that are unlikely in natural language.
//
// Args:
// s: string - Input string.
//
// Returns:
// bool - True if the string contains too many uncommon clusters.
func containsTooManyUncommonClusters(s string) bool {
	uncommonBigrams := map[string]struct{}{
		"qx": {}, "xq": {}, "qj": {}, "jq": {}, "vk": {}, "kj": {}, "zx": {}, "xk": {},
		"vv": {}, "ww": {}, "zz": {}, "qq": {}, "xx": {}, "kk": {}, "jj": {},
		"gf": {}, "fg": {}, "vd": {}, "dv": {}, "qz": {}, "zq": {}, "hj": {}, "jh": {},
	}

	lower := strings.ToLower(s)

	var (
		totalBigrams      int
		uncommonBigramCnt int
		noVowelWindowCnt  int
	)

	for i := 0; i < len(lower)-1; i++ {
		a := lower[i]
		b := lower[i+1]

		if !unicode.IsLetter(rune(a)) || !unicode.IsLetter(rune(b)) {
			continue
		}

		totalBigrams++

		key := string([]byte{a, b})

		if _, exists := uncommonBigrams[key]; exists {
			uncommonBigramCnt++
		}

		if i+4 < len(lower) {
			window := lower[i : i+5]

			if !windowHasVowel(window) {
				noVowelWindowCnt++
			}
		}
	}

	if totalBigrams == 0 {
		return false
	}

	uncommonRatio := float64(uncommonBigramCnt) / float64(totalBigrams)
	noVowelRatio := float64(noVowelWindowCnt) / float64(totalBigrams)

	if uncommonRatio > 0.2 {
		return true
	}

	if noVowelRatio > 0.35 {
		return true
	}

	return false
}

// windowHasVowel checks whether a byte window contains at least one vowel.
//
// Args:
// window: string - Input window.
//
// Returns:
// bool - True if a vowel is present, false otherwise.
func windowHasVowel(window string) bool {
	for _, r := range window {
		if isVowel(r) {
			return true
		}
	}

	return false
}

// countSyllableLikeSegments approximates syllable count by scanning for
// consonant-plus-vowel patterns.
//
// Args:
// s: string - Input string.
//
// Returns:
// int - Approximated number of syllable-like segments.
func countSyllableLikeSegments(s string) int {
	lower := strings.ToLower(s)
	var (
		syllables int
		i         int
		n         = len(lower)
	)

	for i < n {
		for i < n && !isLetterLike(rune(lower[i])) {
			i++
		}

		for i < n && isLetterLike(rune(lower[i])) && !isVowel(rune(lower[i])) {
			i++
		}

		if i < n && isVowel(rune(lower[i])) {
			syllables++

			for i < n && isVowel(rune(lower[i])) {
				i++
			}
		}
	}

	return syllables
}

// hasHighNonLetterDensity checks whether a string contains a high ratio
// of digits and special characters compared to letters.
//
// Args:
// s: string - Input string.
//
// Returns:
// bool - True if non-letter density is too high, false otherwise.
func hasHighNonLetterDensity(s string) bool {
	var letterCount int
	var nonLetterCount int

	for _, r := range s {
		if unicode.IsLetter(r) {
			letterCount++
		} else if unicode.IsDigit(r) || unicode.IsSymbol(r) || unicode.IsPunct(r) {
			nonLetterCount++
		}
	}

	if letterCount == 0 {
		return true
	}

	total := letterCount + nonLetterCount

	if total == 0 {
		return false
	}

	nonLetterRatio := float64(nonLetterCount) / float64(total)

	if nonLetterRatio > 0.3 {
		return true
	}

	return false
}

// hasSuspiciousAlphaNumericRuns checks for long sequences where digits
// are embedded inside otherwise alphabetic chunks.
//
// Args:
// s: string - Input string.
//
// Returns:
// bool - True if suspicious alphanumeric runs are present.
func hasSuspiciousAlphaNumericRuns(s string) bool {
	var (
		currentRunLen int
		hasLetter     bool
		hasDigit      bool
	)

	reset := func() {
		currentRunLen = 0
		hasLetter = false
		hasDigit = false
	}

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			currentRunLen++

			if unicode.IsLetter(r) {
				hasLetter = true
			}

			if unicode.IsDigit(r) {
				hasDigit = true
			}

			if currentRunLen >= 6 && hasLetter && hasDigit {
				return true
			}
		} else {
			reset()
		}
	}

	return false
}
