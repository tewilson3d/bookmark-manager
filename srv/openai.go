package srv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var openaiAPIKey = os.Getenv("OPENAI_API_KEY")

func SetOpenAIKey(key string) {
	openaiAPIKey = key
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
	MaxTokens int            `json:"max_tokens,omitempty"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func summarizeWithLLM(title, description, pageText, url string) (string, error) {
	if openaiAPIKey == "" {
		return "", fmt.Errorf("OpenAI API key not set")
	}

	// Truncate page text to avoid token limits
	if len(pageText) > 4000 {
		pageText = pageText[:4000]
	}

	prompt := fmt.Sprintf(`Summarize this webpage in 1-2 concise sentences. Focus on what it is and why someone would bookmark it.

URL: %s
Title: %s
Description: %s
Page content excerpt: %s

Summary:`, url, title, description, pageText)

	reqBody := openaiRequest{
		Model: "gpt-4o-mini",
		Messages: []openaiMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 150,
	}

	jsonBody, _ := json.Marshal(reqBody)
	
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openaiAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	var result openaiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Error != nil {
		return "", fmt.Errorf("OpenAI error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return result.Choices[0].Message.Content, nil
}
