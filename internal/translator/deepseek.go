package translator

// DeepSeekClient wraps the common client for DeepSeek.
type DeepSeekClient struct {
	*OpenAICompatibleClient
}

// NewDeepSeekClient creates a new client specifically for DeepSeek.
func NewDeepSeekClient(apiURL, apiKey, model string) *DeepSeekClient {
	return &DeepSeekClient{
		OpenAICompatibleClient: NewOpenAICompatibleClient(apiURL, apiKey, model),
	}
}
