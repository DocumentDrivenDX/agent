---
ddx:
  id: SD-005
  depends_on:
    - FEAT-003
    - FEAT-004
    - FEAT-006
    - SD-001
---
# Solution Design: SD-005 — Provider Registry, Model Catalog, and Model-First Routing

## Problem

DDX Agent started with a single flat provider config (`provider`, `base_url`,
`api_key`, `model`). That is sufficient for one local LM Studio instance, but
real users need three separate concerns:

1. **Named providers** — concrete backend definitions for Anthropic,
   OpenRouter, LM Studio hosts, etc.
2. **Shared model policy** — one agent-owned catalog for aliases, numeric
   power scores, per-surface projections, and deprecations.
3. **Simple routing across equivalent providers** — for example choose among
   several local inference servers that should all serve the same requested
   model.

Prompt presets already exist in agent and must remain a separate concern for
system prompt behavior only.

## Design: Two-Layer Resolution Model

DDX Agent keeps two layers above the runtime boundary:

- **Providers** — transport/auth definitions and optional direct pinned models.
- **Model catalog** — agent-owned reusable policy/data loaded from an embedded
  snapshot plus an optional external manifest override, with published manifest
  bundles distributed outside binary releases. Owns model power, cost, context
  window, capability metadata, and reasoning defaults per model.

There is no third "routing config" layer. Per-request routing is **smart
routing** (ADR-005): the service combines the catalog (policy), provider
config (transport), and live signals (provider liveness, provider latency,
subscription quota) to pick the best candidate per request.
Users do not author per-tier candidate lists; they plug in providers, the
service decides.

After resolution, the service builds exactly one concrete native provider
adapter and executes it internally. Consumers do not receive provider
instances.

Caller boundary (see CONTRACT-003):

- Callers choose the harness and pass routing intent through public request
  fields (`Provider`, `Model`, `ModelRef`, `MinPower`, `MaxPower`) plus optional
  auto-selection inputs (`EstimatedPromptTokens`, `RequiresTools`). Explicit
  pins always win over auto-selection.
- Embedded `ddx-agent` chooses the concrete provider candidate, constructs the
  provider adapter, dispatches exactly one candidate, and reports the attempted
  route outcome.
- Callers receive attribution facts from the embedded run (the full ranked
  candidate trace, score components per candidate, and the actual provider
  fired), but do not build providers, inspect private candidate tables, or
  re-inject pre-resolved `RouteDecision` values. `ResolveRoute` results are
  informational only — `Execute` re-resolves on its own inputs.
- Callers own all retry and task-level escalation. If a weak/cheap attempt
  fails, DDx or another caller decides whether to issue a new request with a
  higher power floor; the agent reports route evidence but
  does not dispatch another candidate.

### Config Format

```yaml
# .agent/config.yaml
model_catalog:
  manifest: ~/.config/ddx-agent/models.yaml   # optional local override of the embedded snapshot

providers:
  vidar:
    type: lmstudio
    placement: local
    base_url: http://vidar:1234/v1
    api_key: lmstudio
    reasoning: off

  bragi:
    type: lmstudio
    base_url: http://bragi:1234/v1
    api_key: lmstudio

  grendel:
    type: lmstudio
    base_url: http://grendel:1234/v1
    api_key: lmstudio

  openrouter:
    type: openrouter
    placement: metered
    base_url: https://openrouter.ai/api/v1
    api_key: ${OPENROUTER_API_KEY}
    headers:
      HTTP-Referer: https://github.com/DocumentDrivenDX/agent
      X-Title: DDX Agent

  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}

  vidar-omlx:
    type: omlx
    placement: local
    base_url: http://vidar:1235/v1
    model: Qwen3.5-27B-4bit
    reasoning: off

routing:
  default_model_ref: code-medium    # default catalog tier when caller pins nothing
  health_cooldown: 30s               # how long an unhealthy provider stays excluded

default: vidar                        # fallback provider when no power/model intent is requested
preset: default
max_iterations: 20
session_log_dir: .agent/sessions
```

`model_routes:` is **deprecated** (ADR-005). Legacy configs that still set it
are parsed for one release with a deprecation warning naming the offending
config path; the next release rejects the field outright. Smart routing
covers the same intent automatically: the catalog defines tier membership,
provider config lists endpoints, and the routing engine picks the best
candidate per request without per-tier candidate lists in YAML.

#### `routing.health_cooldown`

*Implementation reference for the routing-quality signal stack; see ADR-006 for the override-tracking layer that feeds operator diagnosis.*

`health_cooldown` is the TTL used by **two** routing signals with different
keying — they share the duration but not the key:

- **Provider cooldowns** (eligibility gate) are keyed by **provider name only**
  (`service_routing.go::buildRoutingInputsWithCatalog` populates
  `routing.Inputs.ProviderCooldowns` from
  `service_route_attempts.go::activeRouteAttempts`). A failed
  `RecordRouteAttempt` for any `(harness, provider, model, endpoint)` tuple
  under that provider name starts a provider-level cooldown that drops the
  entire provider from the candidate set until TTL elapses or any subsequent
  matching success clears it.
- **Per-`(harness, provider, model, endpoint)` success/latency metrics**
  (scoring inputs) use the full tuple key. These do not gate eligibility —
  they only adjust score.

Default TTL: `30s`. Triggers that affect the signal:

1. `RecordRouteAttempt` with `Status="success"` clears matching active
   failures (see clearing semantics below).
2. The `health_cooldown` TTL elapsing since the last failed attempt restores
   eligibility without explicit clearing.
3. No other refresh paths exist in this round.

**Clearing semantics.** A success `RecordRouteAttempt` clears every failure
record whose key matches the success key under wildcard semantics: empty
fields in the success key match any value in the failure key
(`service_route_attempts.go::routeAttemptKeysMatch`). A bare
`{Provider: "alpha"}` success therefore clears **all** failure records
under provider `alpha`; a fully-keyed
`{Harness, Provider, Model, Endpoint}` success clears only that exact
record. This lets harness-level recoveries (`agent doctor` flush, manual
reset) clear a swath of failures with one call without forcing callers to
enumerate every dependent tuple.

#### Provider Config Fields

Per-provider optional fields (in addition to `type`, `base_url`, `api_key`, `headers`, `model`):

| Field | Type | Description |
|---|---|---|
| `reasoning` | scalar string/int | Single public reasoning control: `auto`, `off`, `low`, `medium`, `high`, supported extended values such as `minimal`, `xhigh` / `x-high`, and `max`, or numeric values such as `0`, `2048`, and `8192` |
| `placement` | enum | Optional override for routing placement: `local`, `prepaid`, `metered`, or `test`. Defaults from provider/harness type. Used for placement filtering and cost policy. |
| `max_tokens` | int | Max output tokens per turn; `0` = use provider default |
| `context_window` | int | Explicit context window override; `0` = attempt live discovery |

Older split provider config names are rejected with a clear error. Provider-
specific wire terms such as `thinking`, `effort`, `variant`, and token budgets
are adapter implementation details, not public config.

#### Reasoning Values

`reasoning` is intentionally one scalar rather than separate public level and
budget fields.

- Empty or unset means no caller preference.
- `auto` means resolve model, catalog, or provider defaults.
- `off`, `none`, `false`, and numeric `0` mean explicit reasoning off.
- `low`, `medium`, and `high` use portable fallback budgets of 2048, 8192, and
  32768 tokens when provider/catalog metadata does not publish a better map.
- Extended names such as `minimal`, `xhigh`, `x-high`, and `max` are accepted
  only when the selected provider or harness advertises support. `x-high`
  normalizes to `xhigh`; explicit extended requests are never silently
  downgraded.
- Positive integers mean an explicit max reasoning-token budget, or a
  documented provider-equivalent numeric value.

Providers that only accept numeric reasoning controls must map named values to
numeric budgets with capability-aware model metadata and must enforce
model-specific maximum reasoning-token limits. `max` resolves at the provider
or harness boundary to the selected model/provider maximum and is accepted only
when that maximum is known. Auto/default reasoning controls may be dropped for
unsupported providers/models, but explicit unsupported or over-limit values
fail clearly.

The Go public surface should expose the same single scalar as a typed value,
for example `type Reasoning string` with constants and
`ReasoningTokens(n int) Reasoning`. The implementation should put parsing,
normalization, constants, and policy representation in a shared leaf package
such as `internal/reasoning`; root `agent` may re-export the public type and
helpers, while `internal/modelcatalog` imports the leaf package directly to
avoid root-agent/internal-modelcatalog import cycles.

Model catalog metadata uses `reasoning_default`. Below-smart tiers (`cheap`,
`fast`, `standard`, `code-economy`, and `code-medium`) default to
`reasoning=off`, including local/economy Qwen
targets such as Qwen3.6. `smart` and `code-high` default to `reasoning=high`.
Explicit caller values always win when supported, including numeric values and
values above high such as `xhigh`, `x-high`, or `max`.

### Resolution Model

Per request, the service:

1. Loads provider config and the agent model catalog.
2. Builds an available-model inventory:
   1. Enumerates every configured harness, provider, endpoint, and discovered
      concrete model.
   2. Joins each concrete model to the model catalog. Matched entries provide
      power, family, status, context window, reasoning capability, tool support,
      list price, and benchmark quality. Unknown models remain inspectable but
      are not eligible for automatic routing unless explicitly pinned.
   3. Joins live operational signals: provider health, endpoint cooldown,
      observed latency, prepaid quota remaining/reset time, and known marginal
      cost.
3. Applies caller intent:
   - `--min-power` and `--max-power` select the allowed catalog power range.
     Higher power means stronger for agent tasks. If unset, `MinPower=0` and
     `MaxPower=0` mean "no power bound"; score weights still prefer the best
     available model under cost/availability policy.
   - `--model-ref` is interpreted by catalog kind. A ref to a concrete model
     entry is an exact model constraint. A ref to a power-band alias expands to
     that alias's power bounds; such aliases are human-facing macros, not the
     routing contract.
   - `--model` is an exact concrete model constraint. If the caller asks for
     `qwen-3.6-27b`, the router may choose among providers/endpoints that serve
     that model, but it MUST NOT substitute GPT, Claude, Gemini, or any other
     different model.
   - `--provider` is a hard provider constraint. `--provider lmstudio` means
     only configured LM Studio provider entries/endpoints are considered;
     `--provider openrouter` means only OpenRouter is considered.
   - `--harness` is a hard harness constraint. `--harness codex` means only
     Codex is considered.
   - `--harness + --provider + --model` is concrete and bypasses scoring after
     validation, except for multiple endpoints under the same provider that can
     satisfy the same concrete model.
4. Filters candidates:
   0. Hard constraints remove all candidates outside requested harness,
      provider, and exact-model axes. These constraints are never relaxed by
      power scoring.
   1. Power bounds remove models outside `MinPower..MaxPower` when either bound
      is set. Models without catalog power are removed unless exactly pinned.
   2. Placement policy removes disallowed placements. Local-only placement
      requests never try prepaid or metered providers.
   3. Liveness/model-discovery removes endpoints that are down or do not serve
      the candidate model.
   4. Capability removes candidates with too-small context windows, missing
      tool support for `RequiresTools`, unsupported explicit reasoning, or
      stale/deprecated catalog status when not explicitly allowed.
5. Scores survivors with explicit components:

   ```
   score = power_weighted_capability
         + latency_weight
         + placement_bonus
         + quota_bonus
         - marginal_cost_penalty
         - availability_penalty
         - stale_signal_penalty
   ```

6. Dispatches the top candidate exactly once. On provider/harness failure, the
   service records the attempted route outcome and returns the full ranked
   trace. It does not try the next eligible candidate and it does not widen
   power bounds inside the same request.
7. Falls back to `default:` provider only when no routing intent was supplied
   and the power/catalog path cannot produce a candidate.

The full ranked candidate trace and per-candidate score components are
emitted as part of the routing-decision event (CONTRACT-003) so operators can
explain why candidate 2 lost via `route-status`, not by reading config.

### Failure Evidence and Retry Boundary

The router does not recover by retrying. It has one selection mechanism and one
reporting mechanism:

1. **In-request selection** is service-owned. The service ranks candidates,
   dispatches the top candidate once, and returns the ordered trace.
2. **Retry and escalation** are caller-owned. The caller issues a second request
   with a higher `MinPower`, a different `MaxPower`, or different hard pins when
   its task policy says the extra cost/time is justified.

Every failed routed `Execute` returns enough structured evidence for that
caller decision:

- requested power bounds, hard constraints, and exact pins
- selected candidate, rejected candidates, and filter reasons
- score components and the live/cost/quota facts used for ranking
- final failure class: `setup/config`, `no-candidate`, `provider-transient`,
  `capability`, `cancelled`, or `timeout`
- attempted route outcome for the single dispatched candidate

The catalog may publish human aliases for common bands, but retry logic should
use numeric power. Hard pins do not suggest broader alternatives; if `--model
qwen-3.6-27b` cannot be satisfied, the error explains that exact constraint and
the inspected providers rather than recommending GPT or Claude.

### Available Model Inventory

The service exposes the joined inventory through `DdxAgent.ListModels`. The CLI
MUST expose an operator-facing equivalent, named either
`ddx agent available-models` or `ddx agent models --available` before this
design is implemented. JSON output is the contract; text output is a rendering.

Each row contains:

- identity: harness, provider, endpoint name/base URL, model ID, catalog ID
- policy: power, optional power-band aliases, family, placement, deprecation status
- capability: context window, tool support, reasoning support, streaming and
  structured-output support when known
- economics: placement (`local`, `prepaid`, `metered`, `test`), marginal cost,
  cost source, prepaid quota remaining/reset time
- operations: health, cooldown, recent latency
- routing: power filter reasons and score components for supplied power bounds

This surface is the debugging contract for routing. If `route-status` says a
candidate lost, `available-models --min-power <n> --json` must show the raw
facts that caused the loss.

### Power Bands and Human Aliases

Power is the canonical routing strength axis. Higher values mean stronger
models for agent tasks. Every catalog model must have `power` to be eligible for
automatic routing; newly discovered models without power remain exact-pin only
until the catalog assigns a value.

The catalog may provide human aliases for common ranges, but those aliases are
UI conveniences. The service contract is numeric power.

Implementation prerequisite: the current embedded v4 manifest does not contain
`power`, and the OpenRouter updater only refreshes pricing/context. Before
power routing ships, add catalog schema support and populate power for every
auto-routable model. Bootstrap power from normalized benchmark evidence
(SWE-bench, terminal/TypeScript task benchmarks when available), model
capabilities (context, tools, reasoning), recency, and cost. When benchmark
coverage is missing, cost times recency is the first-order proxy: within a
provider/model family, the newest and most expensive model is assumed strongest
unless the catalog explicitly overrides power or marks an older model as a
useful cost/power exception. Older family members are exact-pin-only for
automatic routing without that override. Keep the raw benchmark inputs beside
the derived power value so catalog updates can evolve scores quantitatively as
new models and measurements arrive.

| Alias | Power bounds | Placement policy | Human intent |
|---|---:|---|---|
| `cheap` | `10..39` | local/free first, then prepaid/cheap metered fallback | Minimize marginal cost for low-risk work. |
| `fast` | `20..49` | local/free or prepaid | Minimize latency for low-risk work. |
| `standard` | `50..79` | local/free first, prepaid fallback | Balanced routine work. |
| `smart` | `80..100` | prepaid frontier first when quota is healthy | Maximize quality. |

Ambiguous aliases are not used by agents for ordering. A caller that wants a
stronger retry raises `MinPower`; a caller that wants to cap spend or latency
sets `MaxPower`. If Claude Code reports usable Opus 4.7 quota and a reset in
five minutes, the effective marginal cost is near zero and a high-power Opus
candidate can rank above weaker local models. If that quota is exhausted, stale,
or far from reset, prepaid bonus is removed and the router may choose a capable
local or metered fallback inside the requested power range.

## Key Design Decisions

**D1: Keep named providers as the concrete transport unit.** Providers hold
endpoint URLs, credentials, and headers. They are not the canonical source of
power or alias policy.

**D2: Add an agent-owned model catalog as a first-class layer.** The catalog is
loaded from an embedded manifest snapshot with an optional external override,
and it owns model power, optional aliases, deprecations, benchmark inputs, and
per-surface projections.

**D2A: Publish catalog bundles independently of binary releases.** The embedded
snapshot remains the safe default, but operators and callers can install a newer
shared manifest from a versioned published bundle via an explicit update flow.

**D2B: Manifest v4 separates concrete models from tier policy.** The model
catalog manifest uses top-level `models` entries for concrete model metadata:
family, display name, parent tier, status, per-million-token costs, cache costs,
context window, benchmark metadata, OpenRouter ID, reasoning budget metadata,
and consumer surface strings. Target entries define only policy: family,
aliases, status/replacement metadata, `context_window_min`, `swe_bench_min`,
ordered `candidates`, and per-surface `reasoning_default`. Older v3 manifests
remain loadable by synthesizing model entries from target surface mappings at
load time.

```yaml
version: 4
models:
  qwen3.5-27b:
    family: qwen
    display_name: Qwen3.5 27B
    tier: code-economy
    status: active
    cost_input_per_m: 0.10
    cost_output_per_m: 0.30
    context_window: 262144
    surfaces:
      agent.openai: qwen3.5-27b
targets:
  code-economy:
    family: coding-tier
    status: active
    context_window_min: 131072
    candidates: [qwen3.5-27b]
    surface_policy:
      agent.openai:
        reasoning_default: off
```

**D3: Preserve prompt preset terminology for prompts only.** The top-level
`preset` field and CLI `--preset` flag refer to system prompt presets defined in
SD-003. Model policy uses `model_ref`, numeric power bounds, alias, or catalog,
never `preset`.

**D4: Smart routing replaces `model_routes`.** Per ADR-005, the service
combines catalog power, provider/harness model inventory, placement,
cost, context, capability, liveness, and usage/quota to
pick the best candidate per request. Users do not author per-tier candidate
lists. `model_routes:` config is deprecated for one release (parsed with a
warning), then rejected outright.

**D5: Power is routing intent; model/provider/harness are constraints.**
`--min-power` and `--max-power` select the model-strength range. Optional
human aliases may expand to power ranges, but agents use numeric power for
ordering and retry decisions. `--model-ref` expands only when it names a catalog
alias; when it names a concrete model entry it is exact. `--model`,
`--provider`, and `--harness` are hard constraints. Routing may optimize cost
and availability inside those constraints but must fail with a detailed
candidate trace when they cannot be met.

**D6: Auto-selection inputs are deterministic.** Auto-selection signals are
`EstimatedPromptTokens` (filter by context window), `RequiresTools` (filter by
tool support), and `Reasoning` (filter by reasoning support). No prose
heuristic complexity classifier. `RequiresTools` is explicit caller intent, or
derived only when a request surface has unambiguously enabled tool execution.

**D7: No agent-owned retry.** The routing engine ranks candidates with explicit
components. `Execute` dispatches the top candidate once and returns the ranked
trace plus attempted-route outcome. DDx or another caller owns any follow-up
request with a stronger `MinPower`, capped `MaxPower`, or different hard pins.
Per-(provider,model,endpoint) availability/latency replaces the per-tier
adaptive min-tier window — one bad endpoint no longer locks out its whole tier.

**D7A: Placement is provider-candidate metadata.** `agent` as a native harness
may front local, prepaid, and metered providers. Routing placement filters
operate on the provider/endpoint candidate, not the harness. Default
placement: `lmstudio`, `omlx`, and `ollama` are `local`; Claude Code, Codex,
and Gemini harnesses are `prepaid` when usable quota evidence exists;
OpenRouter/OpenAI/Anthropic-compatible HTTP providers are `metered` unless
configured otherwise; virtual/script are `test`.

**D8: Environment variable expansion still applies to values.** `${VAR}` is
expanded at config load time. No shell evaluation.

**D9: Backwards compatible with the legacy flat format.** Old flat config
still maps to a single provider named `default`. Legacy `backends`/
`default_backend` and `model_routes:` config are parsed for one release with
a deprecation warning naming the offending key path; the next release
rejects them outright. A boundary test forbids re-introduction of
`model_routes` parsing after the deprecation cycle ends.

**D10: Provider limit discovery is live and flavor-gated.** When
`context_window` or `max_tokens` are zero, the CLI calls `LookupModelLimits`
against the provider's API to discover them. Explicit config values always win.
Discovery is keyed by server flavor:

- **LM Studio** — `GET /api/v0/models/{model}`; prefers `loaded_context_length`
- **omlx** — `GET /v1/models/status`; returns `max_context_window` and
  `max_tokens` per model
- **OpenRouter** — `GET /api/v1/models` (public list)

Undiscoverable values stay zero and the compaction layer uses its own defaults.

**D11: Provider type replaces flavor heuristics for limit discovery.** Port-based provider
detection (e.g. 1234 = lmstudio, 1235 = omlx) fails when servers run on
non-default ports (omlx defaults to 8000). The explicit `type` field lets operators
declare the server type. When type is absent the system:

1. Tries URL-based detection first (reliable for `openrouter.ai`, ollama on
   11434, etc.)
2. Fires concurrent probes to `/v1/models/status` and `/api/v0/models` with a
   3-second timeout to distinguish omlx vs LM Studio on ambiguous ports
3. Falls back to port heuristics as a last resort

**D12: omlx is a first-class supported provider.** omlx is a local inference
runtime that speaks the OpenAI-compatible chat API and exposes additional
endpoints: `GET /v1/models/status` returns per-model `max_context_window` and
`max_tokens`. Set `type: omlx` to use dedicated limit discovery and avoid probe
ambiguity. See the `vidar-omlx` provider entry in the config example above.

**D13: Protocol capabilities are type-keyed and conservative.** The provider
exposes `SupportsTools()`, `SupportsStream()`, and `SupportsStructuredOutput()`
accessors that return the effective capability for the resolved type.
Downstream routing consults these before dispatch to avoid dispatch-and-fail on
mismatched prompts (e.g. 80k-token prompt against a 32k-context model, or
tool-using prompt against a type without tool translation). Unknown types
return `false` for all protocol flags so routing rejects rather than dispatches.
This surface is distinct from the benchmark-based capability scoring used by
smart-routing (`CapabilityScore` / `CapabilityWeight`); the two axes do not
interact.

**`RequiresTools` filter scope.** *Implementation reference; routine `RequiresTools=true` is a routing-quality input, not an override (see ADR-006).* `RequiresTools=true` filters candidates at
the `(harness, provider, model)` level via an **OR-permissive gate**: a
candidate passes when **either** `routing.HarnessEntry.SupportsTools` **or**
`routing.ProviderEntry.SupportsTools` is `true`, AND the catalog's per-model
override (`no_tools: true` in the manifest) is not set. Currently every
builtin harness advertises `SupportsTools=true`
(`service_routing.go::buildRoutingInputsWithCatalog`), so the gate is
effectively provider-and-model driven; the OR exists so a future
tool-incapable harness can still satisfy `RequiresTools` via a tool-capable
provider it fronts.

**D14: `DetectedType()` layers on top of `providerSystem` without replacing
it.** `providerSystem` (URL-heuristic, eager, non-blocking) remains the source
of truth for per-response telemetry and cost attribution because those fire on
every response and cannot afford a network probe. `DetectedType()` is the
probe-confirmed accessor used for pre-dispatch gating (capability flags,
routing tags, introspection). It runs the probe at most once per provider via
`sync.Once`, caches the result, and falls back to `providerSystem` when the
probe is inconclusive. The two accessors serve different audiences by design;
callers of telemetry must not migrate to `DetectedType()` without a
CONTRACT-001 review.

**D15: `reasoning` is the public model-reasoning control.** The public surface
uses one scalar (`reasoning`) for named and numeric values. Config uses
`reasoning`; catalog metadata uses `reasoning_default`; the CLI uses
`--reasoning`. Provider and harness adapters may translate the resolved value
to wire or subprocess knobs named `thinking`, `effort`, `variant`, or numeric
budgets, but those names are not preferred public controls. Unsupported
auto/default controls may be dropped; explicit unsupported or over-limit
values fail clearly.

**D16: Provider model listing is public and endpoint-aware.** `DdxAgent.ListModels`
is the only public surface consumers use to list configured provider-backed
models. For OpenRouter, LM Studio, and oMLX, the service queries each configured
endpoint's `<base_url>/models` endpoint and returns one result per discovered
model per endpoint. The configured provider name, concrete provider type, and
endpoint identity are explicit `ModelInfo` fields so consumers do not read
provider config or infer type from URLs. Endpoint failures are local to that
endpoint during listing; status diagnostics remain in `ListProviders` and
`HealthCheck`.

## CLI UX

### Prompt Preset Selection

The `--preset` flag (or `preset` in config) selects the system prompt style.
Built-in preset names:

| Preset    | Description                                              |
|-----------|----------------------------------------------------------|
| `default` | Balanced, tool-aware prompt                             |
| `smart`   | Rich, thorough prompt for quality-sensitive runs         |
| `cheap`   | Pragmatic, direct prompt for latency/cost-sensitive runs |
| `minimal` | Bare minimum — one sentence                              |
| `benchmark` | Non-interactive prompt optimized for evaluation        |

```bash
ddx-agent -p "prompt"                  # uses preset from config, or "default" by default
ddx-agent -p "prompt" --preset default
ddx-agent -p "prompt" --preset smart
ddx-agent -p "prompt" --preset cheap
```

The `preset` field may also be set in `.agent/config.yaml`:

```yaml
preset: smart
```

Built-in preset details are defined by SD-003 and implemented in
`prompt/presets.go`.

### Direct Provider / Model Selection

```bash
ddx-agent run --provider vidar "prompt"
ddx-agent run --provider anthropic --model opus-4.7 "prompt"
ddx-agent run --model-ref code-high "prompt"
ddx-agent run --model-ref code-high --reasoning max "prompt"
ddx-agent run --model-ref code-medium --reasoning off "prompt"
ddx-agent run --provider vidar --reasoning 8192 "prompt"
```

The public CLI flag is `--reasoning <value>`. Do not introduce alternate public
reasoning flags.

### Smart-Routed Selection

```bash
ddx-agent run --model qwen3.5-27b "prompt"            # pin a concrete model
ddx-agent run --min-power 50 "prompt"                 # require at least medium-strength models
ddx-agent run --min-power 80 "prompt"                 # request stronger models for retry
ddx-agent run "prompt"                                # smart routing across all eligible candidates
```

Compatibility:

```bash
ddx-agent -p "prompt" --backend code-fast-local
```

The compatibility flag remains temporarily, but it is not the preferred UX.

## Library and Package Boundaries

The library runtime boundary does not change: `agent.Run()` still takes a
single `Provider` in the `Request`.

Config and CLI code grow a catalog-aware layer above that boundary. The
detailed package/API shape is defined in
`docs/helix/02-design/plan-2026-04-08-shared-model-catalog.md`,
`docs/helix/02-design/plan-2026-04-10-model-first-routing.md`, and
`docs/helix/02-design/plan-2026-04-10-catalog-distribution-and-refresh.md`.

Expected package split:

- `internal/config/` — load provider config, route config, and optional
  manifest override path in the current repository layout
- `internal/modelcatalog/` — load, validate, and resolve shared model policy
- `internal/reasoning/` — shared leaf package for the Reasoning scalar,
  parser, normalization, constants, `ReasoningTokens(n)`, and resolved policy
  representation
- `cmd/ddx-agent/` — resolve `--provider`, `--model-ref`, or `--model` into
  one concrete provider/model/reasoning policy

## Traceability

- FEAT-004 defines the ownership split and terminology
- SD-003 reserves `preset` for system prompt behavior
- `plan-2026-04-08-shared-model-catalog.md` defines the catalog package/API,
  manifest format, and consumer examples
- `plan-2026-04-10-model-first-routing.md` captures the original model-first
  routing convergence (superseded by ADR-005 for the `model_routes` removal)
- `plan-2026-04-10-catalog-distribution-and-refresh.md` defines published
  manifest bundles, explicit update flow, and the initial reasoning-tier
  baseline
- ADR-005 supersedes D4–D7 with smart routing
- `agent-94b5d420` covers the shared-catalog design lineage
- D10–D12 (provider limit discovery, flavor detection, omlx support) are
  implemented in `internal/config/config.go`, `internal/provider/openai`, and
  the `LookupModelLimits` call-site in the CLI layer
- D15 (reasoning contract) is implemented through `reasoning`,
  `reasoning_default`, and CLI `--reasoning`
- D16 (endpoint-aware provider model listing) is implemented through
  `DdxAgent.ListModels` and the exported `ModelInfo` provider/endpoint fields
