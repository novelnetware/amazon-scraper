package translator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatibleClient is a client for any OpenAI-compatible API.
type OpenAICompatibleClient struct {
	ApiURL     string
	ApiKey     string
	Model      string
	HttpClient *http.Client
}

// NewOpenAICompatibleClient creates a new client instance.
func NewOpenAICompatibleClient(apiURL, apiKey, model string) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		ApiURL: apiURL,
		ApiKey: apiKey,
		Model:  model,
		HttpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

type (
	APIRequest struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
		Stream   bool      `json:"stream"`
	}
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	StreamChunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
)

// TranslateStream sends a prompt and processes the streaming response.
func (c *OpenAICompatibleClient) TranslateStream(ctx context.Context, prompt string) (<-chan string, error) {
	reqBody, err := json.Marshal(APIRequest{
		Model: c.Model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		Stream: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.ApiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.ApiKey)

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("api returned non-200 status: %s, body: %s", resp.Status, string(bodyBytes))
	}

	outChan := make(chan string)

	go func() {
		defer close(outChan)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return // Stop processing if context is cancelled
			default:
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					jsonStr := strings.TrimPrefix(line, "data: ")
					if jsonStr == "[DONE]" {
						return
					}

					var chunk StreamChunk
					if err := json.Unmarshal([]byte(jsonStr), &chunk); err == nil {
						if len(chunk.Choices) > 0 {
							outChan <- chunk.Choices[0].Delta.Content
						}
					}
				}
			}
		}
	}()

	return outChan, nil
}

// Translate provides a simple, non-streaming translation.
func (c *OpenAICompatibleClient) Translate(ctx context.Context, prompt string) (string, error) {
	stream, err := c.TranslateStream(ctx, prompt)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for chunk := range stream {
		result.WriteString(chunk)
	}
	return result.String(), nil
}
