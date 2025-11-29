// Package structs contains the model used by the application
package structs

// Config holds all configuration options for the brainstorm application.
//
// Args:
// nGramMin: int - Minimum n-gram word length.
// nGramMax: int - Maximum n-gram word length.
// outMinLength: int - Minimum output string length.
// outMaxLength: int - Maximum output string length.
// includeNonLatin: bool - When true, relax Latin vowel heuristics to allow multi-byte non-Latin letter sequences.
//
// Returns:
// Config - Configuration object for the application.
type Config struct {
	NGramMin        int
	NGramMax        int
	OutMinLength    int
	OutMaxLength    int
	IncludeNonLatin bool
}
