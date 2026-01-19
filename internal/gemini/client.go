package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/askcli/internal/config"
)

type Client struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type Request struct {
	Contents []Content `json:"contents"`
}

type Content struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type Response struct {
	Candidates    []Candidate    `json:"candidates"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
}

type Candidate struct {
	Content Content `json:"content"`
}

type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// NewClient creates a new Gemini API client
func NewClient(cfg *config.Config) *Client {
	baseURL := cfg.Gemini.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	return &Client{
		apiKey:  cfg.Gemini.APIKey,
		model:   cfg.Gemini.Model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Ask sends a question to Gemini API and returns answer with token information
func (c *Client) Ask(question string, systemMessage string) (string, TokenUsage, error) {
	// Build prompt with system message
	fullPrompt := systemMessage + "\n\nQuestion: " + question

	// Create request
	reqBody := Request{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: fullPrompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("error creating request: %w", err)
	}

	// Build URL
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("error creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("error executing request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("error reading response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", TokenUsage{}, fmt.Errorf("API error (code %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp Response
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", TokenUsage{}, fmt.Errorf("error parsing response: %w", err)
	}

	// Extract answer text
	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return "", TokenUsage{}, fmt.Errorf("empty response from API")
	}

	answer := apiResp.Candidates[0].Content.Parts[0].Text

	// Clean markdown code blocks from answer
	answer = cleanMarkdownCodeBlocks(answer)

	// Extract token information
	var tokenUsage TokenUsage
	if apiResp.UsageMetadata != nil {
		tokenUsage = TokenUsage{
			InputTokens:  apiResp.UsageMetadata.PromptTokenCount,
			OutputTokens: apiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:  apiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return answer, tokenUsage, nil
}

// cleanMarkdownCodeBlocks removes markdown code block markers from text
func cleanMarkdownCodeBlocks(text string) string {
	// Remove code block markers like ```bash, ```sh, ```, etc.
	// Pattern matches: ``` followed by optional language identifier and newline, then content, then closing ```
	re := regexp.MustCompile("(?s)```[a-zA-Z]*\\n(.*?)```")
	text = re.ReplaceAllString(text, "$1")

	// Also handle cases where there might be just ``` on separate lines
	text = strings.ReplaceAll(text, "```bash\n", "")
	text = strings.ReplaceAll(text, "```sh\n", "")
	text = strings.ReplaceAll(text, "```\n", "")
	text = strings.ReplaceAll(text, "```bash", "")
	text = strings.ReplaceAll(text, "```sh", "")
	text = strings.ReplaceAll(text, "```", "")

	// Clean up extra newlines
	text = strings.TrimSpace(text)

	return text
}
