package router

import (
	"testing"

	"ai-gateway/internal/provider"
)

func TestClassifyComplexitySimple(t *testing.T) {
	req := &provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "What is the weather today?"},
		},
	}
	c := classifyComplexity(req)
	if c != ComplexitySimple {
		t.Errorf("expected simple, got %s", c)
	}
}

func TestClassifyComplexityLongMessage(t *testing.T) {
	long := ""
	for i := 0; i < 2001; i++ {
		long += "x"
	}
	req := &provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: long},
		},
	}
	c := classifyComplexity(req)
	if c != ComplexityComplex {
		t.Errorf("expected complex for long message, got %s", c)
	}
}

func TestClassifyComplexityCode(t *testing.T) {
	req := &provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Write a function in Go:\n```\nfunc hello() {\n  fmt.Println(\"hi\")\n}\n```"},
		},
	}
	c := classifyComplexity(req)
	if c != ComplexityComplex {
		t.Errorf("expected complex for code, got %s", c)
	}
}

func TestClassifyComplexityMultiStep(t *testing.T) {
	req := &provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "system", Content: "You must think step by step and explain your reasoning carefully."},
			{Role: "user", Content: "Solve this problem."},
		},
	}
	c := classifyComplexity(req)
	if c != ComplexityComplex {
		t.Errorf("expected complex for multi-step, got %s", c)
	}
}

func TestClassifyComplexityDefKeyword(t *testing.T) {
	req := &provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "def hello(): return 'world'"},
		},
	}
	c := classifyComplexity(req)
	if c != ComplexityComplex {
		t.Errorf("expected complex for 'def' keyword, got %s", c)
	}
}
