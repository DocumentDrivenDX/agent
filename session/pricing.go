package session

import "github.com/DocumentDrivenDX/forge"

// ModelPricing holds per-million-token costs for a model.
// Alias of forge.ModelPricing — kept here for backward compatibility.
type ModelPricing = forge.ModelPricing

// PricingTable maps model IDs to their pricing.
// Alias of forge.PricingTable — kept here for backward compatibility.
type PricingTable = forge.PricingTable

// DefaultPricing contains built-in pricing for common models.
// Delegates to forge.DefaultPricing so both packages share one source of truth.
var DefaultPricing = forge.DefaultPricing
