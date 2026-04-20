package openai

// Protocol-level capability flags describe what the concrete provider can
// honor on the OpenAI-compatible wire: tool calls, streaming, and structured
// output modes. They are distinct from routing-layer capability (a
// benchmark-quality score used by smart routing). Callers use these flags to
// gate dispatch before sending unsupported request shapes.

// ProtocolCapabilities declares provider-owned protocol capability claims.
type ProtocolCapabilities struct {
	Tools            bool
	Stream           bool
	StructuredOutput bool
	// Thinking reports whether the provider accepts the non-standard `thinking`
	// body field (openai-go WithJSONSet) that LM Studio and a few compatible
	// servers tolerate.
	Thinking bool
}

var (
	OpenAIProtocolCapabilities  = ProtocolCapabilities{Tools: true, Stream: true, StructuredOutput: true, Thinking: false}
	UnknownProtocolCapabilities ProtocolCapabilities
)

func (p *Provider) protocolCapabilities() ProtocolCapabilities {
	if p.capabilities != nil {
		return *p.capabilities
	}
	return OpenAIProtocolCapabilities
}

// SupportsTools reports whether the concrete provider accepts a `tools`
// field on `/v1/chat/completions` and returns structured `tool_calls` in the
// response.
func (p *Provider) SupportsTools() bool {
	return p.protocolCapabilities().Tools
}

// SupportsStream reports whether `stream: true` returns a well-formed SSE
// stream with incremental `choices[0].delta` chunks.
func (p *Provider) SupportsStream() bool {
	return p.protocolCapabilities().Stream
}

// SupportsStructuredOutput reports whether the provider honors
// `response_format: json_object` / tool-use-required semantics to produce a
// structured (JSON-shaped) response.
func (p *Provider) SupportsStructuredOutput() bool {
	return p.protocolCapabilities().StructuredOutput
}

// SupportsThinking reports whether the provider accepts the non-standard
// `thinking` request-body field used to cap reasoning-token budgets on
// LM Studio and compatible servers. Providers returning false MUST have the
// field stripped at serialization time.
func (p *Provider) SupportsThinking() bool {
	return p.protocolCapabilities().Thinking
}
