package processor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// TranslationClient برای ارتباط با API ترجمه استفاده می‌شود.
type TranslationClient struct {
	APIUrl     string
	APIKey     string
	Model      string
	HttpClient *http.Client
}

// FIX: مطمئن شوید که نام تابع با حرف بزرگ 'N' شروع می‌شود.
// NewTranslationClient یک کلاینت جدید برای ترجمه می‌سازد.
func NewTranslationClient(apiURL, apiKey, model string) *TranslationClient {
	return &TranslationClient{
		APIUrl:     apiURL,
		APIKey:     apiKey,
		Model:      model,
		HttpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// ساختارهای مورد نیاز برای درخواست و پاسخ API
type APIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type APIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// TranslateText متن انگلیسی را به فارسی روان ترجمه می‌کند.
func (c *TranslationClient) TranslateText(textToTranslate string, prompt string) (string, error) {
	if textToTranslate == "" {
		return "", nil
	}

	fullPrompt := prompt + "\n\n" + textToTranslate

	requestBody, err := json.Marshal(APIRequest{
		Model: c.Model,
		Messages: []Message{
			{Role: "user", Content: fullPrompt},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.APIUrl, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", errors.New("API request failed with status: " + resp.Status + " and body: " + string(bodyBytes))
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", err
	}

	if len(apiResp.Choices) > 0 {
		return apiResp.Choices[0].Message.Content, nil
	}

	return "", errors.New("no translation content received from API")
}
