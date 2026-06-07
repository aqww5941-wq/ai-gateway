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

type OpenAIProvider struct {
	name    string
	apiKey  string
	baseURL string
	models  []string
	client  *http.Client
	logger  *slog.Logger
}

func NewOpenAI(cfg config.ProviderConfig, logger *slog.Logger) (*OpenAIProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai provider %q: api_key is empty", cfg.Name)
	}
	if len(cfg.Models) == 0 {
		return nil, fmt.Errorf("openai provider %q: models list is empty", cfg.Name)
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &OpenAIProvider{
		name:    cfg.Name,
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		models:  cfg.Models,
		client:  &http.Client{Timeout: timeout},
		logger:  logger,
	}, nil
}

func (p *OpenAIProvider) Name() string             { return p.name }
func (p *OpenAIProvider) SupportedModels() []string { return p.models }

func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req.Stream {
		return nil, fmt.Errorf("streaming not supported")
	}
	return p.doChatCompletion(ctx, req)
}

func (p *OpenAIProvider) ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan *StreamChunk, error) {
	req.Stream = true
	httpReq, err := p.buildRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("upstream error: status=%d body=%s", resp.StatusCode, string(body))
	}

	ch := make(chan *StreamChunk, 16)
	go p.readSSEStream(ctx, resp, ch)
	return ch, nil
}

func (p *OpenAIProvider) buildRequest(ctx context.Context, req *ChatRequest) (*http.Request, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	return httpReq, nil
}

func (p *OpenAIProvider) doChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	httpReq, err := p.buildRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	p.logger.Debug("calling upstream", "url", httpReq.URL.String(), "model", req.Model)

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
		return nil, fmt.Errorf("upstream error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &chatResp, nil
}

func (p *OpenAIProvider) readSSEStream(ctx context.Context, resp *http.Response, ch chan *StreamChunk) {
	defer resp.Body.Close()
	defer close(ch)

	reader := io.Reader(resp.Body)
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 256)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := reader.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		for {
			idx := indexOf(buf, []byte("\n\n"))
			if idx < 0 {
				break
			}
			line := buf[:idx]
			buf = buf[idx+2:]

			if bytes.HasPrefix(line, []byte("data: ")) {
				data := line[6:]
				if string(data) == "[DONE]" {
					return
				}
				var chunk StreamChunk
				if err := json.Unmarshal(data, &chunk); err != nil {
					p.logger.Warn("failed to parse SSE chunk", "error", err)
					continue
				}
				select {
				case ch <- &chunk:
				case <-ctx.Done():
					return
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func indexOf(b []byte, sub []byte) int {
	for i := 0; i <= len(b)-len(sub); i++ {
		if bytes.Equal(b[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}
