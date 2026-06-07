package observer

var modelPrices = map[string]struct {
	promptPrice     float64
	completionPrice float64
}{
	"gpt-4o":              {2.50, 10.00},
	"gpt-4o-mini":         {0.15, 0.60},
	"deepseek-chat":       {0.14, 0.28},
	"deepseek-reasoner":   {0.55, 2.19},
	"claude-3-5-sonnet":   {3.00, 15.00},
	"claude-3-haiku":      {0.25, 1.25},
}

// CalculateCost computes the cost in USD based on token usage.
// Prices are per 1M tokens.
func CalculateCost(model string, promptTokens, completionTokens int) float64 {
	price, ok := modelPrices[model]
	if !ok {
		return 0
	}
	promptCost := float64(promptTokens) / 1_000_000.0 * price.promptPrice
	completionCost := float64(completionTokens) / 1_000_000.0 * price.completionPrice
	return promptCost + completionCost
}
