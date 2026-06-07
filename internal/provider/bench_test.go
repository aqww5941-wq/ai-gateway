package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
)

// BenchmarkReadSSEStream measures the cost of parsing a typical OpenAI-style
// SSE response with 200 chunks of ~30 bytes each (representative of a short
// chat completion). The bufio.Scanner-based parser is O(N); the previous
// implementation rescanned the buffer on every chunk and was O(N^2).
func BenchmarkReadSSEStream(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
	_ = os.Stderr
	payload := genSSEPayload(200)
	p := &OpenAIProvider{logger: logger}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp := &http.Response{
			Body:       io.NopCloser(bytes.NewReader([]byte(payload))),
			StatusCode: 200,
		}
		ch := make(chan *StreamChunk, 256)
		go p.readSSEStream(context.Background(), resp, ch)
		for range ch {
		}
	}
}

// BenchmarkReadSSEStream_Large stresses the parser with a long stream
// (2000 chunks). On the original O(N^2) implementation this scaled
// quadratically (10x chunks → 100x time); on the Scanner-based one
// it scales linearly.
func BenchmarkReadSSEStream_Large(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
	payload := genSSEPayload(2000)
	p := &OpenAIProvider{logger: logger}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp := &http.Response{
			Body:       io.NopCloser(bytes.NewReader([]byte(payload))),
			StatusCode: 200,
		}
		ch := make(chan *StreamChunk, 4096)
		go p.readSSEStream(context.Background(), resp, ch)
		for range ch {
		}
	}
}

func genSSEPayload(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb,
			"data: {\"id\":\"x\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"tok-%d\"}}]}\n\n",
			i,
		)
	}
	sb.WriteString("data: [DONE]\n\n")
	return sb.String()
}

// --- Baseline: the original O(N^2) implementation, kept for benchmark
//     comparison only. Do not use in production. ---

func indexOfNaive(b, sub []byte) int {
	for i := 0; i <= len(b)-len(sub); i++ {
		if bytes.Equal(b[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

func readSSEStreamOld(ctx context.Context, resp *http.Response, ch chan *StreamChunk) {
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
			idx := indexOfNaive(buf, []byte("\n\n"))
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
				if jerr := jsonUnmarshal(data, &chunk); jerr != nil {
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

// jsonUnmarshal is a thin shim so the old function references the same
// json package as the new one. Calling json.Unmarshal directly would force
// us to import "encoding/json" only for the baseline benchmark.
func jsonUnmarshal(data []byte, v any) error {
	return jsonUnmarshalImpl(data, v)
}

func BenchmarkReadSSEStream_Old_Small(b *testing.B) {
	payload := genSSEPayload(200)
	for i := 0; i < b.N; i++ {
		resp := &http.Response{Body: io.NopCloser(bytes.NewReader([]byte(payload))), StatusCode: 200}
		ch := make(chan *StreamChunk, 256)
		go readSSEStreamOld(context.Background(), resp, ch)
		for range ch {
		}
	}
}

func BenchmarkReadSSEStream_Old_Large(b *testing.B) {
	payload := genSSEPayload(2000)
	for i := 0; i < b.N; i++ {
		resp := &http.Response{Body: io.NopCloser(bytes.NewReader([]byte(payload))), StatusCode: 200}
		ch := make(chan *StreamChunk, 4096)
		go readSSEStreamOld(context.Background(), resp, ch)
		for range ch {
		}
	}
}
