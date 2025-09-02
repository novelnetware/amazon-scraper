package translator

// OpenAIClient wraps the common client for OpenAI.
type OpenAIClient struct {
	*OpenAICompatibleClient
}

// NewOpenAIClient creates a new client specifically for OpenAI.
func NewOpenAIClient(apiURL, apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		OpenAICompatibleClient: NewOpenAICompatibleClient(apiURL, apiKey, model),
	}
}
