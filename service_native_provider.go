package agent

import (
	agentcore "github.com/DocumentDrivenDX/agent/internal/core"
	"github.com/DocumentDrivenDX/agent/internal/provider/anthropic"
	"github.com/DocumentDrivenDX/agent/internal/provider/lmstudio"
	"github.com/DocumentDrivenDX/agent/internal/provider/ollama"
	"github.com/DocumentDrivenDX/agent/internal/provider/omlx"
	oaiProvider "github.com/DocumentDrivenDX/agent/internal/provider/openai"
	"github.com/DocumentDrivenDX/agent/internal/provider/openrouter"
)

func (s *service) resolveConfiguredNativeProvider(req ServiceExecuteRequest) agentcore.Provider {
	sc := s.opts.ServiceConfig
	if sc == nil {
		return nil
	}
	name := req.Provider
	if name == "" {
		name = sc.DefaultProviderName()
	}
	if name == "" {
		return nil
	}
	entry, ok := sc.Provider(name)
	if !ok {
		return nil
	}
	if req.Model != "" {
		entry.Model = req.Model
	}
	return buildNativeProvider(name, entry)
}

func buildNativeProvider(name string, entry ServiceProviderEntry) agentcore.Provider {
	switch normalizeServiceProviderType(entry.Type) {
	case "anthropic":
		return anthropic.New(anthropic.Config{
			BaseURL: entry.BaseURL,
			APIKey:  entry.APIKey,
			Model:   entry.Model,
		})
	case "lmstudio":
		return lmstudio.New(lmstudio.Config{
			BaseURL: entry.BaseURL,
			APIKey:  entry.APIKey,
			Model:   entry.Model,
		})
	case "openrouter":
		return openrouter.New(openrouter.Config{
			BaseURL: entry.BaseURL,
			APIKey:  entry.APIKey,
			Model:   entry.Model,
		})
	case "omlx":
		return omlx.New(omlx.Config{
			BaseURL: entry.BaseURL,
			APIKey:  entry.APIKey,
			Model:   entry.Model,
		})
	case "ollama":
		return ollama.New(ollama.Config{
			BaseURL: entry.BaseURL,
			APIKey:  entry.APIKey,
			Model:   entry.Model,
		})
	case "openai", "minimax", "qwen", "zai":
		return oaiProvider.New(oaiProvider.Config{
			BaseURL:        entry.BaseURL,
			APIKey:         entry.APIKey,
			Model:          entry.Model,
			ProviderName:   name,
			ProviderSystem: normalizeServiceProviderType(entry.Type),
		})
	default:
		return nil
	}
}
