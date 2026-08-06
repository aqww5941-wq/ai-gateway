package provider

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-gateway/config"
)

const fixtureProviderCredential = "fixture-provider-credential"

type observedOpenAIRequest struct {
	method      string
	path        string
	contentType string
	authOK      bool
	body        ChatRequest
}

func TestOpenAIProviderUnaryHTTPContract(t *testing.T) {
	observed := make(chan observedOpenAIRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "invalid fixture request", http.StatusBadRequest)
			return
		}
		observed <- observedOpenAIRequest{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			authOK:      r.Header.Get("Authorization") == "Bearer "+fixtureProviderCredential,
			body:        body,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ChatResponse{
			ID:      "fixture-response-id",
			Object:  "chat.completion",
			Model:   body.Model,
			Choices: []Choice{{Index: 0, Message: Message{Role: "assistant", Content: "fixture-output"}, FinishReason: "stop"}},
			Usage:   Usage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6},
		})
	}))
	t.Cleanup(upstream.Close)

	provider := newContractTestOpenAI(t, upstream.URL)
	temperature := 0.25
	maxTokens := 32
	response, err := provider.ChatCompletion(context.Background(), &ChatRequest{
		Model:       "fixture-upstream-model",
		Messages:    []Message{{Role: "user", Content: "fixture-input"}},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		t.Fatal(err)
	}

	request := <-observed
	if request.method != http.MethodPost || request.path != "/chat/completions" {
		t.Fatalf("upstream target = %s %s, want POST /chat/completions", request.method, request.path)
	}
	if request.contentType != "application/json" || !request.authOK {
		t.Fatalf("upstream headers = content-type %q, authorization valid %v", request.contentType, request.authOK)
	}
	if request.body.Model != "fixture-upstream-model" || request.body.Stream || len(request.body.Messages) != 1 || request.body.Messages[0].Content != "fixture-input" {
		t.Fatalf("upstream body = %#v", request.body)
	}
	if request.body.Temperature == nil || *request.body.Temperature != temperature || request.body.MaxTokens == nil || *request.body.MaxTokens != maxTokens {
		t.Fatalf("sampling fields were not preserved: %#v", request.body)
	}
	if response.ID != "fixture-response-id" || response.Model != "fixture-upstream-model" || response.Usage.TotalTokens != 6 || response.Choices[0].Message.Content != "fixture-output" {
		t.Fatalf("decoded response = %#v", response)
	}
}

func TestOpenAIProviderClassifiesHTTPErrorForUnaryAndStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		http.Error(w, `{"error":{"code":"fixture_rate_limited"}}`, http.StatusTooManyRequests)
	}))
	t.Cleanup(upstream.Close)

	for _, testCase := range []struct {
		name string
		call func(*OpenAIProvider) error
	}{
		{
			name: "unary",
			call: func(provider *OpenAIProvider) error {
				_, err := provider.ChatCompletion(context.Background(), &ChatRequest{Model: "fixture-upstream-model"})
				return err
			},
		},
		{
			name: "stream",
			call: func(provider *OpenAIProvider) error {
				_, err := provider.ChatCompletionStream(context.Background(), &ChatRequest{Model: "fixture-upstream-model"})
				return err
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call(newContractTestOpenAI(t, upstream.URL))
			upstreamError := AsUpstream(err)
			if upstreamError == nil {
				t.Fatalf("error type = %T, want *UpstreamError", err)
			}
			if upstreamError.Provider != "fixture-openai" || upstreamError.StatusCode != http.StatusTooManyRequests || upstreamError.RetryAfter != 2*time.Second || !upstreamError.IsRetryable() {
				t.Fatalf("classified upstream error = %#v", upstreamError)
			}
			if upstreamError.BodyLen == 0 || !strings.Contains(upstreamError.Body, "fixture_rate_limited") {
				t.Fatalf("upstream error body metadata = length %d", upstreamError.BodyLen)
			}
			if strings.Contains(err.Error(), fixtureProviderCredential) {
				t.Fatal("classified error contains the provider credential")
			}
		})
	}
}

func TestOpenAIProviderSSEReplayStopsAtDone(t *testing.T) {
	observed := make(chan observedOpenAIRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode stream request: %v", err)
			return
		}
		observed <- observedOpenAIRequest{method: r.Method, path: r.URL.Path, authOK: r.Header.Get("Authorization") == "Bearer "+fixtureProviderCredential, body: body}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, ": fixture-heartbeat\r\n\r\n")
		_, _ = io.WriteString(w, "data: {\"id\":\"fixture-stream\",\"object\":\"chat.completion.chunk\",\"model\":\"fixture-upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"fixture-\"}}]}\r\n\r\n")
		_, _ = io.WriteString(w, "data: {\"id\":\"fixture-stream\",\"object\":\"chat.completion.chunk\",\"model\":\"fixture-upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"output\"}}],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":2,\"total_tokens\":6}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\r\n\r\n")
		_, _ = io.WriteString(w, "data: {\"id\":\"must-not-be-read\"}\n\n")
	}))
	t.Cleanup(upstream.Close)

	provider := newContractTestOpenAI(t, upstream.URL)
	chunks, err := provider.ChatCompletionStream(context.Background(), &ChatRequest{Model: "fixture-upstream-model"})
	if err != nil {
		t.Fatal(err)
	}
	var received []*StreamChunk
	for chunk := range chunks {
		received = append(received, chunk)
	}

	request := <-observed
	if request.method != http.MethodPost || request.path != "/chat/completions" || !request.authOK || !request.body.Stream {
		t.Fatalf("stream request contract = %#v", request)
	}
	if len(received) != 2 {
		t.Fatalf("stream chunks = %d, want 2 before [DONE]", len(received))
	}
	if received[0].Choices[0].Delta.Role != "assistant" || received[0].Choices[0].Delta.Content != "fixture-" || received[1].Choices[0].Delta.Content != "output" {
		t.Fatalf("stream chunks out of order: %#v", received)
	}
	if received[1].Usage == nil || received[1].Usage.TotalTokens != 6 {
		t.Fatalf("final stream usage = %#v", received[1].Usage)
	}
}

func TestOpenAIProviderStreamCancellationReachesUpstream(t *testing.T) {
	upstreamCanceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = io.WriteString(w, "data: {\"id\":\"fixture-stream\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"first\"}}]}\n\n")
		flusher.Flush()
		<-r.Context().Done()
		close(upstreamCanceled)
	}))
	t.Cleanup(upstream.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	provider := newContractTestOpenAI(t, upstream.URL)
	chunks, err := provider.ChatCompletionStream(ctx, &ChatRequest{Model: "fixture-upstream-model"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case chunk, open := <-chunks:
		if !open || chunk == nil || len(chunk.Choices) != 1 || chunk.Choices[0].Delta.Content != "first" {
			t.Fatalf("first stream chunk = %#v, open %v", chunk, open)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first stream chunk")
	}
	cancel()

	select {
	case <-upstreamCanceled:
	case <-time.After(5 * time.Second):
		t.Fatal("request cancellation did not reach the upstream")
	}
	select {
	case _, open := <-chunks:
		if open {
			t.Fatal("stream channel remained open after cancellation")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream channel did not close after cancellation")
	}
}

func newContractTestOpenAI(t *testing.T, baseURL string) *OpenAIProvider {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	provider, err := NewOpenAI(config.ProviderConfig{
		Name:    "fixture-openai",
		APIKey:  fixtureProviderCredential,
		BaseURL: baseURL,
		Models:  []string{"fixture-upstream-model"},
		Timeout: 5 * time.Second,
	}, logger)
	if err != nil {
		t.Fatal(err)
	}
	return provider
}
