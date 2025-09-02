package translator

import "context"

// TranslateResult holds the outcome of a translation.
type TranslateResult struct {
	Text  string
	Error error
}

// Translator defines a generic interface for any AI translation service.
// This allows us to easily switch between providers like DeepSeek and OpenAI.
type Translator interface {
	// TranslateStream sends a prompt to the AI and returns the response as a stream of text chunks.
	TranslateStream(ctx context.Context, prompt string) (<-chan string, error)

	// Translate performs a simple, non-streaming translation.
	Translate(ctx context.Context, prompt string) (string, error)
}
