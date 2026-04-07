---
ddx:
  id: SD-005
  depends_on:
    - FEAT-003
    - FEAT-004
    - FEAT-006
    - SD-001
---
# Solution Design: SD-005 — Multi-Provider Configuration

## Problem

Forge currently supports a single provider configured via flat fields
(`provider`, `base_url`, `api_key`, `model`). Real users need multiple
providers configured simultaneously — local LM Studio on different hosts,
OpenRouter for cloud fallback, Anthropic direct, etc. — and need to switch
between them easily.

## Design: Named Provider Registry

### Config Format

```yaml
# .forge/config.yaml
providers:
  local:
    type: openai-compat
    base_url: http://localhost:1234/v1
    model: qwen3.5-7b

  vidar:
    type: openai-compat
    base_url: http://vidar:1234/v1
    model: qwen/qwen3-coder-next

  bragi:
    type: openai-compat
    base_url: http://bragi:1234/v1
    model: qwen3.5-27b

  openrouter:
    type: openai-compat
    base_url: https://openrouter.ai/api/v1
    api_key: ${OPENROUTER_API_KEY}
    model: anthropic/claude-sonnet-4
    headers:
      HTTP-Referer: https://github.com/DocumentDrivenDX/forge
      X-Title: Forge

  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    model: claude-sonnet-4-20250514

default: local
max_iterations: 20
session_log_dir: .forge/sessions
```

### Key Design Decisions

**D1: Named providers, not a flat list.** Names are user-chosen identifiers
(not provider types). A user can have `local`, `vidar`, `bragi` all as
`openai-compat` type with different URLs. Names are used on the CLI:
`forge -p "prompt" --provider vidar`.

**D2: Environment variable expansion in values.** `${OPENROUTER_API_KEY}` is
expanded from the environment at config load time. This keeps secrets out of
the config file. Only `${VAR}` syntax — no shell evaluation.

**D3: Extra headers for OpenRouter et al.** The `headers` map is passed through
on every HTTP request. This supports OpenRouter's `HTTP-Referer` and `X-Title`,
Azure's custom headers, or any other provider-specific headers.

**D4: Backwards compatible.** The old flat format still works:
```yaml
provider: openai-compat
base_url: http://localhost:1234/v1
model: qwen3.5-7b
```
This is equivalent to a single provider named `default`. If both `providers:`
and flat fields exist, `providers:` takes precedence.

**D5: `default` key selects the provider when no `--provider` flag is given.**
If omitted, the first provider in the map is the default (YAML map order).

**D6: `forge providers` lists with live status.** Hits each provider's
`/v1/models` endpoint (or equivalent) to show connectivity. This replaces the
current `forge check` which only checks one provider.

### CLI UX

```
forge providers                      # list all configured providers with status
forge providers --json               # machine-readable

forge models                         # list models for default provider
forge models --provider vidar        # list models for specific provider
forge models --all                   # list models for ALL providers

forge check                          # check default provider (backwards compat)
forge check --provider openrouter    # check specific provider
forge check --all                    # check all providers

forge -p "prompt" --provider vidar   # use specific provider
forge -p "prompt"                    # use default provider
```

### Output Examples

```
$ forge providers
NAME         TYPE           URL                              MODEL                      STATUS
local        openai-compat  http://localhost:1234/v1          qwen3.5-7b                 unreachable
vidar        openai-compat  http://vidar:1234/v1             qwen/qwen3-coder-next      connected (3 models)
bragi        openai-compat  http://bragi:1234/v1             qwen3.5-27b                connected (10 models)
openrouter   openai-compat  https://openrouter.ai/api/v1     anthropic/claude-sonnet-4   connected
anthropic    anthropic      https://api.anthropic.com        claude-sonnet-4-20250514    api key set
* default: vidar

$ forge models --provider vidar
qwen/qwen3-coder-next
minimax/minimax-m2.5
text-embedding-nomic-embed-text-v1.5

$ forge models --all
[vidar]
qwen/qwen3-coder-next
minimax/minimax-m2.5

[bragi]
qwen3.5-27b
qwen/qwen3.5-9b
qwen/qwen3-coder-30b
google/gemma-4-26b-a4b
...

[openrouter]
(845 models — use forge models --provider openrouter to list)

[anthropic]
claude-sonnet-4-20250514 (configured)
```

### Library API

The library doesn't change much. `forge.Run()` still takes a `Provider` in
the `Request`. The multi-provider config is CLI-layer concern — the CLI resolves
the named provider to a `forge.Provider` instance and passes it in.

For library users who want the config system:

```go
// Package config provides multi-provider configuration loading.
package config

type ProviderConfig struct {
    Type    string            `yaml:"type"`    // "openai-compat" or "anthropic"
    BaseURL string            `yaml:"base_url"`
    APIKey  string            `yaml:"api_key"`
    Model   string            `yaml:"model"`
    Headers map[string]string `yaml:"headers"`
}

type Config struct {
    Providers     map[string]ProviderConfig `yaml:"providers"`
    Default       string                    `yaml:"default"`
    MaxIterations int                       `yaml:"max_iterations"`
    SessionLogDir string                    `yaml:"session_log_dir"`
}

// Load reads config from .forge/config.yaml and ~/.config/forge/config.yaml
// with env var expansion.
func Load(workDir string) (*Config, error)

// BuildProvider creates a forge.Provider from a named provider config.
func (c *Config) BuildProvider(name string) (forge.Provider, error)

// DefaultProvider creates the default provider.
func (c *Config) DefaultProvider() (forge.Provider, error)

// ProviderNames returns configured provider names in order.
func (c *Config) ProviderNames() []string
```

### OpenAI Provider: Headers Support

Add an optional `Headers` field to the OpenAI provider config:

```go
type Config struct {
    BaseURL string
    APIKey  string
    Model   string
    Headers map[string]string // extra HTTP headers (OpenRouter, Azure, etc.)
}
```

These are passed as `option.WithHeader(k, v)` on every request.

### Migration from Old Config

The `Load()` function detects the old flat format and converts it:

```yaml
# Old format
provider: openai-compat
base_url: http://localhost:1234/v1
model: qwen3.5-7b
```

Becomes internally:

```yaml
providers:
  default:
    type: openai-compat
    base_url: http://localhost:1234/v1
    model: qwen3.5-7b
default: default
```

No user action needed. Old configs keep working.

## Implementation Plan

| # | Bead | Depends |
|---|------|---------|
| 1 | Config types + loader with env expansion and migration | — |
| 2 | OpenAI provider: add Headers support | — |
| 3 | `forge providers` command | 1 |
| 4 | `forge models --provider` + `--all` | 1, 2 |
| 5 | `forge check` update for multi-provider | 1 |
| 6 | Wire `--provider` flag into `forge -p` | 1 |

## Risks

| Risk | Prob | Impact | Mitigation |
|------|------|--------|------------|
| YAML map ordering | L | L | Go yaml.v3 preserves insertion order |
| Env var expansion edge cases | L | L | Only `${VAR}` syntax, no shell eval |
| OpenRouter model list very large | M | L | Truncate with count in `--all` mode |
