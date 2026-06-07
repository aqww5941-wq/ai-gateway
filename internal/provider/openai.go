package provider

import (
	"bufio"
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
		return nil, newUpstreamErrorFromResp(p.name, resp.StatusCode, body, resp.Header.Get("Retry-After"), maxErrorBodyBytes)
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
	// Inject W3C traceparent so the upstream call appears as a child span in
	// the caller's trace. injectTraceContext is a no-op when tracing is off.
	injectTraceContext(ctx, httpReq.Header)
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
		return nil, newUpstreamErrorFromResp(p.name, resp.StatusCode, respBody, resp.Header.Get("Retry-After"), maxErrorBodyBytes)
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

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1<<20) // 1 MiB max event size
	scanner.Split(splitSSEEvent)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		event := scanner.Bytes()
		// An SSE event may have multiple "data:" lines; the OpenAI/DeepSeek
		// stream sends one chunk per event, so we look for the first one.
		for _, line := range bytes.Split(event, []byte("\n")) {
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			data := bytes.TrimSpace(line[5:])
			if len(data) == 0 {
				continue
			}
			if bytes.Equal(data, []byte("[DONE]")) {
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
	if err := scanner.Err(); err != nil && err != io.EOF {
		p.logger.Warn("sse scanner error", "error", err)
	}
}

// splitSSEEvent splits the input on blank-line event delimiters (\n\n or \r\n\r\n).
// It returns one full event per call without allocating new slices for the body.
func splitSSEEvent(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
		return i + 2, data[:i], nil
	}
	if i := bytes.Index(data, []byte("\r\n\r\n")); i >= 0 {
		return i + 4, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
