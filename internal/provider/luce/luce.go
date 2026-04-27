// Package luce wraps the OpenAI-compat shape exposed by the lucebox-hub
// dflash server (https://github.com/Luce-Org/lucebox-hub). The server runs
// hand-tuned CUDA inference (DFlash speculative decoding + DDTree verify)
// behind an OpenAI-compat HTTP API on :1236. For routing and benchmarking
// purposes treat luce like any other openai-compat local provider — same
// shape as lmstudio (:1234) and omlx (:1235), just a different runtime
// underneath.
//
// Capabilities mirror lmstudio: tool calling, streaming, and structured
// output are exposed on the OpenAI-compatible surface. Reasoning controls
// are not declared on the wire (the server does not surface
// enable_thinking / reasoning_effort), so Thinking stays false; if a
// future server release adds them, flip the flag and pick the matching
// ThinkingWireFormat.
//
// Sampling: the server accepts the standard OpenAI sampler fields. Catalog
// entries for luce-served models should not set sampling_control unless a
// specific model id is observed to pin samplers server-side; in the absence
// of that signal, leaving sampling_control unset (= client_settable) lets
// internal/sampling.Resolve push catalog profile values to the wire as
// usual.
//
// Default port 1236 fits alongside the existing :1234 (lmstudio) and :1235
// (omlx) conventions on the LAN.
package luce

import (
	"github.com/DocumentDrivenDX/agent/internal/provider/openai"
	"github.com/DocumentDrivenDX/agent/internal/reasoning"
)

const DefaultBaseURL = "http://localhost:1236/v1"

// ProtocolCapabilities mirrors lmstudio's openai-compat surface — the
// routing engine treats luce as a full participant rather than a narrow
// special case. Per-model exceptions (e.g., a model that pins samplers
// server-side) belong on the catalog ModelEntry, not here.
var ProtocolCapabilities = openai.ProtocolCapabilities{
	Tools:            true,
	Stream:           true,
	StructuredOutput: true,
}

type Config struct {
	BaseURL      string
	APIKey       string
	Model        string
	ModelPattern string
	KnownModels  map[string]string
	Headers      map[string]string
	Reasoning    reasoning.Reasoning
}

func New(cfg Config) *openai.Provider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return openai.New(openai.Config{
		BaseURL:        baseURL,
		APIKey:         cfg.APIKey,
		Model:          cfg.Model,
		ProviderName:   "luce",
		ProviderSystem: "luce",
		ModelPattern:   cfg.ModelPattern,
		KnownModels:    cfg.KnownModels,
		Headers:        cfg.Headers,
		Reasoning:      cfg.Reasoning,
		Capabilities:   &ProtocolCapabilities,
	})
}
