---
ddx:
  id: ADR-005
  depends_on:
    - CONTRACT-003
    - SD-005
---
# ADR-005: Smart Routing Replaces `model_routes`

| Date | Status | Deciders | Related | Confidence |
|------|--------|----------|---------|------------|
| 2026-04-25 | Proposed | DDX Agent maintainers | `CONTRACT-003`, `SD-005` | Medium |

## Context

SD-005 currently makes `model_routes:` the resolution surface: users hand-author per-tier candidate lists in YAML, the CLI re-reads them and synthesizes a `RouteDecision` injected through `ServiceExecuteRequest.PreResolved`, and the service treats that as authoritative. The block exists to coordinate same-tier failover among local LM Studio hosts and to keep the routing engine from stripping configured candidates whose discovery probe is failing.

This is two failure modes welded together:

1. **Configurable failover in user YAML** is the wrong surface. The model catalog already knows which models occupy each tier (`code-economy`/`code-medium`/`code-high`/`smart`), provider config already lists endpoints, and the routing engine already scores `(harness, provider, model)` candidates with cost / latency / capability inputs. Forcing users to also write `model_routes:` makes them coordinate three sources of truth that the service could coordinate itself.

2. **The CLI synthesis path is leaky.** `cmd/agent/main.go:474-487` builds a `RouteDecision{Reason: "cli configured route"}`, threads it through `ServiceExecuteRequest.PreResolved`, and overwrites the request's `Provider`/`Model`/`Harness` fields — which the contract claims `PreResolved` mode ignores. The mechanism only exists because the routing engine strips configured candidates on probe failure; without that strip, the CLI synthesis would not be needed.

Two adjacent observed behaviors confirm the design is wrong:

- When all configured local providers are down (`vidar`, `grendel` 502/timeout), the engine returns "all tiers exhausted — no viable provider found" instead of falling forward to a healthy subscription harness (`claude-max`, `codex-pro`). The user's expectation is automatic fallback when quota allows.
- The `adaptive min-tier` heuristic locks out the `cheap` tier after a low trailing-window success rate (observed: 0.06 over 17 attempts), and the lockout never recovers because no cheap-tier attempts run to refresh the signal.

The shape we want: providers are transport, the catalog is policy, the routing engine decides per request based on liveness, prompt characteristics, and a cost/latency/capability score. Users do not maintain a routing table.

## Decision

Replace the `model_routes`-driven resolution surface with deterministic smart routing.

### 2026-04-30 clarification: profile-driven candidate inventory

This ADR's original wording was too easy to implement as "profile resolves to
one model, then score providers for that one model." That is not the intended
router. A profile is a policy bundle that tells the service where to start in
the model-selection tree, which placements are allowed, and which score weights
matter most. It is not a concrete model alias.

The service MUST build a complete candidate inventory before choosing:

1. **Provider/harness inventory** — enumerate every available execution surface:
   prepaid subscription harnesses (`claude`, `codex`, `gemini` when quota
   evidence says usable), native provider endpoints (`lmstudio`, `omlx`,
   `ollama`, OpenRouter/OpenAI-compatible endpoints), and test harnesses only
   when explicitly requested.
2. **Model inventory** — ask each surface what concrete models it can serve.
   Live `/models` or harness discovery output wins; configured provider
   defaults are fallback hints, not the whole inventory.
3. **Catalog join** — match discovered concrete models to catalog entries.
   Catalog metadata supplies tier, family, context window, reasoning support,
   tool support, quality benchmarks, deprecation status, and list price.
4. **Usage/cost join** — attach live usage/quota signals where the surface can
   provide them. Prepaid harnesses expose quota remaining and reset time; paid
   metered providers expose static or live cost; local/free providers expose
   zero marginal cost plus measured latency/reliability when known.
5. **Inspectable output** — expose this joined inventory through a public CLI
   surface (`ddx agent available-models --json`, or an equivalent
   `ddx agent models --available --json`) and the service `ListModels` API.
   Operators must be able to see the same candidate table the router scores:
   harness, provider, endpoint, model, tier, family, placement, cost class,
   marginal cost, quota/reset, context, tool support, reasoning support,
   health, recent latency, recent success rate, and filter reasons.

Profiles then filter and weight that inventory:

| Profile | Catalog floor | Placement policy | Primary weights |
|---|---:|---|---|
| `local`, `offline`, `air-gapped` | `code-economy` | local/free only | cost, availability, latency |
| `cheap` | `code-economy` | local/free first; prepaid/cheap metered fallback | cost, availability, reliability |
| `fast` | `code-medium` | local/free or prepaid, whichever is low-latency and capable | latency, reliability, cost |
| `standard` | `code-medium` | local/free first when capable; prepaid fallback | reliability, cost, latency |
| `smart` | `code-high` | prepaid frontier first when quota is healthy; local fallback only when frontier/prepaid is unavailable or explicitly cheaper for equivalent quality | capability, reliability, quota, latency |

The "catalog floor" is a minimum quality tier, not a single target model. For
example, `smart` filters out models below `code-high` and should normally rank
current frontier Opus/GPT-class models above older or economy models. `cheap`
starts at `code-economy` and may select a strong local model when it is live,
tool-capable, and cheap to run.

Selection is a transparent utility calculation, not a hidden preference:

```
score = profile_weighted_capability
      + profile_weighted_reliability
      + profile_weighted_latency
      + placement_bonus
      + quota_bonus
      - marginal_cost_penalty
      - cooldown_penalty
      - stale_signal_penalty
```

Prepaid quota changes the marginal-cost term. If Claude Code reports usable
Opus quota with a reset in five minutes, `smart` may rank Opus first because
the effective marginal cost is near zero and the quality score is high. If the
same quota is exhausted, stale, or near a long reset horizon, the quota bonus
disappears and cost/availability penalties apply. Local LM Studio/oMLX/Ollama
providers are treated as free marginal cost but still compete on capability,
tool support, context, latency, and recent success.

Provider placement is candidate-level. The native `agent` harness is not
itself "local" or "subscription"; its child provider endpoints are. A single
native harness may contain local oMLX, local LM Studio, and paid OpenRouter
providers, and profile filtering must operate on those provider candidates.

Failover uses the same ordered candidate trace and never escapes hard caller
constraints. `Execute` tries the best candidate first, records the attempt,
then tries the next eligible candidate on transient provider/harness failures
only when that next candidate still satisfies the requested harness, provider,
and exact-model constraints. It must not fail over deterministic request errors
such as invalid prompt envelopes, malformed tool schemas, unsupported explicit
pins, or configuration parse failures. Provider-specific authentication or
quota failures are transient for that candidate and may fail over to another
endpoint/provider only when the caller did not hard-constrain that axis; global
missing configuration is not.

### 1. Auto-selection rules

`Execute` auto-fills only the axes the caller left unconstrained. A `Profile`
is broad routing policy. `Harness`, `Provider`, and exact model identity are
hard constraints:

- `Harness=claude` means only the Claude harness may be used.
- `Provider=lmstudio` means only LM Studio providers/endpoints may be used.
- `Model=qwen-3.6-27b` means only that model, including provider-native aliases
  that fuzzy-match the same catalog model, may be used. The router may optimize
  provider/endpoint choice inside that model constraint, but it must not select
  a different model such as GPT-5 mini.

A `ModelRef` is interpreted by catalog type: refs that resolve to a concrete
model entry are exact model constraints; refs that resolve to a target/profile
(`cheap`, `standard`, `smart`, `code-medium`, etc.) expand to that target's
candidate models. If a constrained request cannot be satisfied, routing fails
with a detailed candidate/error trace instead of broadening the constraint.

Default profile is `smart` when no profile/model intent is supplied.

Auto-selection signals are deterministic and already available:

- `EstimatedPromptTokens` — prompt size in tokens. Used to filter candidates whose context window cannot hold the prompt.
- `RequiresTools` — whether the request requires tool calls. It is explicit
  caller intent; automatic derivation is allowed only when the request surface
  has unambiguously enabled tool execution. Text-only requests do not become
  tool-requiring merely because a harness can use tools.
- `Reasoning` — caller's reasoning request. Used to filter providers whose support level is below the request.

These existed in `internal/routing.Request` already (`internal/routing/engine.go:15`); the gap is that public `RouteRequest`/`ServiceExecuteRequest` did not surface them, so service-side smart routing was blind. ADR adds them to the public surface (see CONTRACT-003 update).

No prose-heuristic complexity classifier. Token count plus `RequiresTools` is the entire signal in this round.

### 2. Routing decision

Per request:

1. **Build the candidate set** = every available `(harness, provider,
   endpoint, model)` joined with the catalog and live provider/harness signals,
   then apply hard caller constraints before scoring. The requested profile's
   catalog floor filters out models below the minimum tier; it does not
   collapse the set to one primary model unless the caller supplied an exact
   model constraint.
2. **Filter by liveness** via `HealthCheck` and live model discovery. Drop
   endpoints whose latest probe failed or which do not advertise the candidate
   model. If the filter empties the set, escalate the catalog floor only for
   profiles whose policy allows escalation. Local-only profiles never escalate
   into subscription/cloud placement.
3. **Filter by capability**: drop candidates whose context window <
   `EstimatedPromptTokens`, whose `SupportsTools()` is false when
   `RequiresTools` is true, whose reasoning support is below the request, or
   whose catalog tier/family is below the profile floor.
4. **Score each survivor** using explicit score components: catalog quality,
   recent success rate, observed latency, marginal cost, quota/reset state,
   placement preference, and cooldown/staleness penalties. Candidate trace
   output must expose these components.
5. **Dispatch top-1**, return the full ranked candidate trace in the routing decision event so callers can see why candidates 2..N lost.
6. **On failure rotate** within the same tier; only escalate the tier when the same-tier set is exhausted. Record outcome to update per-(provider,model) stats. **Replaces the per-tier trailing-window adaptive min-tier** (which was too coarse — locked the cheap tier out forever after 17 failed attempts because no cheap attempts could refresh the signal).

#### Pipeline order

Steps 1–6 above describe the user-visible flow. The implementation collapses them into the engine's two phases:

**In `routing.Resolve` (`internal/routing/engine.go`):** consume a fully joined
candidate inventory → apply inline gates (liveness via provider/endpoint
cooldown, capability via `EstimatedPromptTokens` / `RequiresTools` /
`Reasoning`, placement policy, subscription quota, catalog tier floor, harness
allowlist) → score eligible candidates with cost, latency, capability,
reliability, placement, and quota signals → rank and tie-break by cost and
latency.

**In `service.ResolveRoute` (`service_routing.go`):** wrap the engine with profile-tier escalation when the engine returns `ErrNoLiveProvider`. Catalog tier filtering and profile ceiling enforcement happen in the engine's inline gates as part of candidate construction; cross-tier escalation lives at the service layer because it loops `routing.Resolve` over successive ladder profiles.

#### Escalation ladder

When same-tier candidates are exhausted (all filtered or all scored
ineligible), `service.ResolveRoute` walks the profile escalation chain declared
by the profile policy. Built-in default: `cheap → standard → smart`.
Escalation is **one-way upward only**. Profiles with local-only placement
(`local`, `offline`, `air-gapped`) do not escalate into subscription/cloud.
Custom profiles escalate only when their catalog profile or future policy block
declares a next profile; absence of that declaration means no escalation.

### 3. Per-(provider, model) success/latency

In-memory + TTL only this round (matches today's `service_route_attempts.go:13`). Persistent state across restarts is deferred until storage and warm-start behavior are designed. Key change vs. today: signal is keyed on `(provider, model)`, not on tier. A single bad model does not lock out its whole tier.

### 4. Subscription quota inputs

Claude/Codex/Gemini already publish quota signals via harness caches (`service_routing.go:335`). Cost ramping when ≥80% used already exists. Keep both unchanged.

OpenRouter and native HTTP providers do not publish live quota. Treat their cost as static catalog cost in this round; file a follow-up bead for live-quota plumbing on those providers but do not block this work on it.

### 5. `route-status` redesigned

Today `route-status` enumerates configured `model_routes` keys. Post-deletion it must report **eligible candidates for a requested intent or profile**, with score components (quality, cost, latency, success-rate, filter reason) per candidate, and the per-(provider,model) success/latency stats. Operators read it to answer "why did the router pick X?" — not to inspect their own YAML.

### 6. Delete

- `model_routes:` config block; its loader in `internal/config/config.go`; `ServiceConfig.ModelRouteConfig`/`ModelRouteNames`.
- `service_routing.go` model_routes short-circuit landed in `90d9b03` (revert).
- `ServiceExecuteRequest.PreResolved` and `RouteDecision`-as-input. `PreResolved` was specified for a dry-run-then-execute flow that has no current consumer; its only producer in the repo is the CLI synthesis at `cmd/agent/main.go:474-487`, which is itself part of the `model_routes` deletion. `ResolveRoute` remains as a public method (operator dashboard / debug surface), but its result is informational, not re-injectable.
- CLI `selection.RouteCandidates` and `cmd/agent/routing_provider.go` provider-construction wrappers.
- SD-005 D4–D7 (model-route surface). SD-005 rewritten from this ADR.

### 7. Keep

- `routing.default_model`, `routing.default_model_ref`, `routing.health_cooldown` config keys. These are useful defaults, not model_routes.
- `internal/modelcatalog` — source of truth for tier policy, cost, context, capability.
- `internal/routing` engine scoring — refactor input source, do not rewrite scoring.
- Provider adapters, `internal/reasoning`, the three session-log refactors landed earlier in this stack (`agent-7faa0edf`, `agent-b9bd700f`, `agent-99549438`).
- `--profile cheap|fast|smart`, `--model`, `--provider`, `--reasoning`, `--model-ref` CLI flags.

### 8. Backward compatibility

For one release: parse `model_routes:` if present, log a deprecation warning naming the offending config path, **honor the configured ordering**. Hard-erroring immediately is safer than silently ignoring (warn-and-ignore is the worst option — semantic drift). Remove the parser and the warning in the next release.

Add a `cmd/agent/service_boundary_test.go` structural check that fails if `internal/config` reintroduces `model_routes` parsing after the deprecation cycle ends.

## Consequences

### Positive

- One source of routing truth (catalog + provider config + engine), not three.
- Live-provider fallback works automatically: when local LM Studio hosts are down and subscription quota is available, requests route to `claude-max`/`codex-pro` without operator config.
- Per-(provider,model) signal recovers from transient failures; one bad model no longer locks out its tier indefinitely.
- `RouteCandidate` exposes structured score components, not a free-form `Reason` string. Operator debugging gets a real surface.
- Public `RouteRequest` exposes the prompt-aware inputs the engine already needed; service-side smart routing is no longer blind.

### Negative

- Removes a configurable failover surface. Power users who deliberately wire an ordered candidate list lose that knob. Mitigation: explicit `--provider <name>` and `--model <name>` pins remain; chaining failover by ordering candidates was already a workaround for the engine's probe-strip behavior, which this ADR fixes at the source.
- Public surface change to `RouteRequest`/`ServiceExecuteRequest` (new fields; one removed). Consumers re-bind.
- One-release deprecation window means operators with `model_routes:` configs do not get an immediate hard error. Acceptable trade-off vs. silent drift.

## Migration

Plan in three sharper beads (replacing the obsolete chain `agent-9d120ece`/`6dd4ad97`/`873081a9`/`8804194f`, which is canceled with note "superseded by ADR-005"):

1. **Public surface update** — add `EstimatedPromptTokens` / `RequiresTools` to `RouteRequest` and `ServiceExecuteRequest`; remove `ServiceExecuteRequest.PreResolved`; add structured score components to `RouteCandidate`; update CONTRACT-003. Revert `90d9b03`. Update SD-005 with the auto-selection section and deprecation note.

2. **Wire inputs + scoring + route-status** — plumb new `RouteRequest` fields from CLI through `Execute`; wire engine gates against them; expose score components in routing-decision events; redesign `route-status` to show eligible candidates per intent. Add per-(provider,model) success/latency keying. Replace per-tier adaptive min-tier with per-model signal.

3. **Config + CLI cleanup + deprecation** — delete `model_routes` parser and `ServiceConfig.ModelRouteConfig`; delete CLI `selection.RouteCandidates` synthesis and `routing_provider.go` provider-construction wrappers; add deprecation warning when parsing legacy config; add boundary test forbidding `model_routes` re-entry.

Beads in steps 2 and 3 can be parallelized across two workers; step 1 blocks both.

## Out of scope (deferred)

- Persistent EWMA across process restarts. In-memory + TTL is fine for this round; persistence + warm-start is its own design.
- ML-style prompt classification beyond `EstimatedPromptTokens`/`RequiresTools`. Ship deterministic smart routing first.
- Live quota plumbing for OpenRouter and native HTTP providers. Static catalog cost suffices in this round.
- Reviewer pipeline overflow fixes — tracked separately in upstream `ddx` repo (FEAT-022 + `ddx-021bd69b`); this repo's only related work is one bead in step 1 to tighten the success-final usage convention.

## Related

- `CONTRACT-003` — public service surface; updated in step 1.
- `SD-005` — provider/model/routing config; rewritten from this ADR.
- `internal/routing/engine.go` — existing scoring engine; input source refactored, scoring unchanged.
- `service_routing.go` — subscription quota cost ramp at line 593 stays; `90d9b03` short-circuit reverts.
- Upstream `ddx-021bd69b` — reviewer JSON verdict contract (sibling repo, separate fix path).
