package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"ai-gateway/config"
)

type ClaudeProvider struct {
	name    string
	apiKey  string
	baseURL string
	models  []string
	client  *http.Client
	logger  *slog.Logger
}

func NewClaude(cfg config.ProviderConfig, logger *slog.Logger) (*ClaudeProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("claude provider %q: api_key is empty", cfg.Name)
	}
	if len(cfg.Models) == 0 {
		return nil, fmt.Errorf("claude provider %q: models list is empty", cfg.Name)
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	return &ClaudeProvider{
		name:    cfg.Name,
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		models:  cfg.Models,
		client:  &http.Client{Timeout: timeout},
		logger:  logger,
	}, nil
}

func (p *ClaudeProvider) Name() string              { return p.name }
func (p *ClaudeProvider) SupportedModels() []string { return p.models }

func (p *ClaudeProvider) SetTransport(rt *http.Transport) {
	if rt != nil {
		p.client.Transport = rt
	}
}

// --- Anthropic request/response types ---

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Model      string                  `json:"model"`
	Content    []anthropicContentBlock `json:"content"`
	Usage      anthropicUsage          `json:"usage"`
	StopReason string                  `json:"stop_reason"`
}

func (p *ClaudeProvider) ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan *StreamChunk, error) {
	return nil, fmt.Errorf("claude streaming not yet supported")
}

func (p *ClaudeProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req.Stream {
		return nil, fmt.Errorf("streaming not supported")
	}

	ar, system := toAnthropicRequest(req)

	body, err := json.Marshal(ar)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	injectTraceContext(ctx, httpReq.Header)

	p.logger.Debug("calling claude upstream", "url", p.baseURL+"/messages", "model", ar.Model)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, newUpstreamErrorFromResp(p.name, resp.StatusCode, respBody, resp.Header.Get("Retry-After"), maxErrorBodyBytes)
	}

	var arResp anthropicResponse
	if err := json.Unmarshal(respBody, &arResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	chatResp := toChatResponse(&arResp, system)
	return chatResp, nil
}

func toAnthropicRequest(req *ChatRequest) (*anthropicRequest, string) {
	var system string
	messages := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			// Anthropic uses a top-level system field instead of a message.
			if system != "" {
				system += "\n"
			}
			system += m.Content
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "assistant"
		}
		messages = append(messages, anthropicMessage{Role: role, Content: m.Content})
	}

	maxTokens := 1024
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	return &anthropicRequest{
		Model:       req.Model,
		Messages:    messages,
		System:      system,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Stream:      false,
	}, system
}

func toChatResponse(ar *anthropicResponse, _ string) *ChatResponse {
	content := ""
	for _, block := range ar.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	finishReason := ""
	switch ar.StopReason {
	case "end_turn":
		finishReason = "stop"
	case "max_tokens":
		finishReason = "length"
	case "stop_sequence":
		finishReason = "stop"
	}

	return &ChatResponse{
		ID:      ar.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   ar.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: Usage{
			PromptTokens:     ar.Usage.InputTokens,
			CompletionTokens: ar.Usage.OutputTokens,
			TotalTokens:      ar.Usage.InputTokens + ar.Usage.OutputTokens,
		},
	}
}
