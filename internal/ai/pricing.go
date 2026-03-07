package ai

// ModelPricing holds per-million-token pricing for a model.
type ModelPricing struct {
	InputPerMillion  float64
	OutputPerMillion float64
}

// PricingTable maps model names/prefixes to pricing.
var PricingTable = map[string]ModelPricing{
	"claude-opus-4":       {InputPerMillion: 15.00, OutputPerMillion: 75.00},
	"claude-opus-4-6":     {InputPerMillion: 15.00, OutputPerMillion: 75.00},
	"claude-sonnet-4":     {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"claude-sonnet-4-6":   {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"claude-haiku-4-5":    {InputPerMillion: 0.80, OutputPerMillion: 4.00},
	"gpt-4o":              {InputPerMillion: 2.50, OutputPerMillion: 10.00},
	"gpt-4.1":             {InputPerMillion: 2.00, OutputPerMillion: 8.00},
	"gpt-4.1-mini":        {InputPerMillion: 0.40, OutputPerMillion: 1.60},
	"gpt-4.1-nano":        {InputPerMillion: 0.10, OutputPerMillion: 0.40},
	"o3":                  {InputPerMillion: 10.00, OutputPerMillion: 40.00},
	"o3-mini":             {InputPerMillion: 1.10, OutputPerMillion: 4.40},
	"o4-mini":             {InputPerMillion: 1.10, OutputPerMillion: 4.40},
}

// LookupPricing returns the pricing for a model. Returns zero pricing if not found.
func LookupPricing(model string) (ModelPricing, bool) {
	if p, ok := PricingTable[model]; ok {
		return p, true
	}
	return ModelPricing{}, false
}

// CalculateCost computes the USD cost for a given model and token counts.
func CalculateCost(model string, inputTokens, outputTokens int64) float64 {
	pricing, ok := LookupPricing(model)
	if !ok {
		return 0
	}
	return (float64(inputTokens)*pricing.InputPerMillion +
		float64(outputTokens)*pricing.OutputPerMillion) / 1_000_000
}
