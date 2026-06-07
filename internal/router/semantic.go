package router

import (
	"context"
	"fmt"
	"strings"

	"ai-gateway/config"
	"ai-gateway/internal/provider"
)

type Complexity string

const (
	ComplexitySimple  Complexity = "simple"
	ComplexityComplex Complexity = "complex"
)

func classifyComplexity(req *provider.ChatRequest) Complexity {
	totalLen := 0
	hasCode := false
	hasMultiStep := false

	for _, m := range req.Messages {
		totalLen += len(m.Content)
		if m.Role == "system" && containsMultiStep(m.Content) {
			hasMultiStep = true
		}
		if containsCodeBlock(m.Content) {
			hasCode = true
		}
	}

	// Rule 1: total message length > 2000 chars → complex
	if totalLen > 2000 {
		return ComplexityComplex
	}
	// Rule 2: system message requires multi-step reasoning → complex
	if hasMultiStep {
		return ComplexityComplex
	}
	// Rule 3: contains code blocks → complex
	if hasCode {
		return ComplexityComplex
	}

	return ComplexitySimple
}

func containsMultiStep(s string) bool {
	keywords := []string{
		"step by step", "step-by-step", "reasoning",
		"think carefully", "explain your reasoning",
		"first", "then", "finally",
	}
	count := 0
	for _, kw := range keywords {
		if strings.Contains(strings.ToLower(s), kw) {
			count++
		}
	}
	return count >= 2
}

func containsCodeBlock(s string) bool {
	return strings.Contains(s, "```") ||
		strings.Contains(s, "func ") ||
		strings.Contains(s, "def ") ||
		strings.Contains(s, "class ") ||
		strings.Contains(s, "import ")
}

type SemanticStrategy struct {
	rules map[Complexity]Target
}

func NewSemanticStrategy(cfgs []config.SemanticRuleConfig) (*SemanticStrategy, error) {
	rules := make(map[Complexity]Target)
	for _, c := range cfgs {
		complexity := Complexity(c.Complexity)
		if complexity != ComplexitySimple && complexity != ComplexityComplex {
			return nil, fmt.Errorf("unknown complexity %q", c.Complexity)
		}
		rules[complexity] = Target{
			Provider: c.Target.Provider,
			Model:    c.Target.Model,
		}
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("semantic strategy requires at least one rule")
	}
	return &SemanticStrategy{rules: rules}, nil
}

func (s *SemanticStrategy) Select(ctx context.Context, req *provider.ChatRequest, targets []Target) (*Target, error) {
	c := classifyComplexity(req)
	if target, ok := s.rules[c]; ok {
		return &target, nil
	}
	// Fallback to the first rule
	for _, t := range s.rules {
		return &t, nil
	}
	return nil, fmt.Errorf("no matching semantic rule")
}
