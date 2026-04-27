# Provider caching survey — 2026-04-27

Scope: what we send on the wire to opt into prompt caching for each provider surface
the agent harness uses, and what telemetry confirms a hit. Local-server sections focus
on whether KV reuse persists across separate HTTP requests (the relevant case for an
agent harness, which makes one `/v1/chat/completions` call per turn).

## 1. Anthropic Messages API

- **Cache type**: explicit prefix prompt cache (server-side, ephemeral KV).
- **Opt-in**: `cache_control: {"type": "ephemeral"}` placed on the *last* block of any
  cacheable region. Allowed regions: `tools[i]`, `system[i]`, `messages[i].content[j]`.
  TTL options: default `"5m"`, or `"1h"` (extra `"ttl": "1h"` field; gated by
  `extra-cache-ttl-2025-04-11` beta header until GA — currently still required as of
  the public docs we could fetch).
- **Max breakpoints**: 4 explicit per request. Top-level `cache_control` for "automatic"
  caching also consumes a slot.
- **Telemetry**: `usage.cache_creation_input_tokens`, `usage.cache_read_input_tokens`,
  and (with mixed TTLs) `usage.cache_creation.ephemeral_5m_input_tokens` /
  `ephemeral_1h_input_tokens`. Total input = read + creation + input.
- **Min prompt length** (must be at or after the breakpoint, cumulatively):
  - Opus 4.5/4.6/4.7, Haiku 4.5: **4096 tokens**
  - Sonnet 4.6: 2048
  - Sonnet 4.5/4/3.7, Opus 4.0/4.1, Haiku 3.5: 1024
  - Below threshold: silently no-cache (both usage fields = 0).
- **Lookback**: server walks back **up to 20 blocks** from each breakpoint.
- **Traps**:
  - Breakpoint placed *after* a varying token (timestamps in the system prompt,
    UUIDs in tool descriptions) defeats the cache.
  - Feb 5 2026 isolation change: caches are now **workspace-scoped**, not org-scoped,
    on Claude API and Azure AI Foundry. Bedrock/Vertex remain org-scoped. This affects
    cache reuse across workspaces in the same org.
  - Thinking blocks cannot carry `cache_control` directly; they piggyback on the
    surrounding assistant turn.

**Verdict on our implementation** (`internal/provider/anthropic/anthropic.go`):
`convertTools` stamps ephemeral on the last tool, `buildSystemBlocks` stamps ephemeral
on the last system block. That is the correct, minimum-friction layout for two
breakpoints (tools-prefix + system-prefix). However:

- We do **not** stamp a third breakpoint on the last assistant/tool-result turn, so
  multi-turn conversation prefixes are *not* explicitly cached. For agent loops with
  long tool transcripts this leaves savings on the table — the turn-N prefix should
  carry a moving breakpoint placed on the last tool-result of turn N-1.
- We do not expose a 1h TTL knob; everything is 5m default.
- `convertMessages` rebuilds tool_use/tool_result blocks from `agent.Message` each call.
  As long as `tc.ID`, `tc.Name`, and serialized `tc.Arguments` are byte-stable across
  retries, prefix stability holds. Worth a wire-level conformance test that replays the
  same turn twice and asserts `cache_read_input_tokens > 0`.

Source: <https://platform.claude.com/docs/en/docs/build-with-claude/prompt-caching>.

## 2. OpenAI Chat Completions (GPT-5.x, codex)

- **Cache type**: automatic server-side prefix cache. No client opt-in required.
- **Opt-in**: none. Optional `prompt_cache_key` (string) request field acts as a
  shard hint; OpenAI routes requests with the same key to the same backend to keep
  hit rate up under load (~15 RPM/prefix per machine before overflow).
- **Telemetry**: `usage.prompt_tokens_details.cached_tokens` (int). Always present,
  zero on miss or sub-1024-token prompts.
- **Constraints**: minimum **1024 prompt tokens**; cache hits then advance in 128-token
  increments. Eligible on gpt-4o and newer (gpt-5, gpt-5.1, gpt-5.2-codex, etc.).
- **Discount**: ~50% on cached input tokens.
- **Traps**: prefix must be byte-identical. Ordering of tools/system matters. Without
  `prompt_cache_key`, hot prefixes scatter across machines under concurrency.

**Verdict on our implementation**: we do not currently send `prompt_cache_key`. For
single-user CLI use this is fine; for any concurrent harness (beadbench parallelism)
we should set it to a stable per-session value. We do not record `cached_tokens` in
session telemetry — worth surfacing.

Sources: <https://developers.openai.com/api/docs/guides/prompt-caching>,
<https://openai.com/index/api-prompt-caching/>.

## 3. OpenRouter

- **Cache type**: pass-through to the upstream provider.
- **Opt-in**:
  - **Anthropic models** behind OpenRouter: send `cache_control` exactly as you would
    to Anthropic (per-block ephemeral, or top-level for automatic). OpenRouter
    forwards. 1h TTL supported.
  - **OpenAI / Grok / Moonshot / Groq / DeepSeek**: automatic; nothing to send.
  - **Gemini 2.5**: implicit caching is on by default. Optional explicit `cache_control`
    breakpoints in content blocks; OpenRouter only forwards the **last** breakpoint.
- **Telemetry**: normalised across providers in `usage`:
  - `usage.cache_discount` (float, USD, sign-aware: negative on writes, positive on reads).
  - `usage.prompt_tokens_details.cached_tokens` (int) for OpenAI-family upstreams.
  - For Anthropic upstream: pass-through `cache_creation_input_tokens` /
    `cache_read_input_tokens`.
- **Provider sticky routing**: OpenRouter pins subsequent requests in a session to the
  same upstream endpoint when caching is detected, to maximise hit rate.
- **Traps**:
  - Top-level (automatic) `cache_control` only works when the request is routed to
    Anthropic-direct, not Bedrock/Vertex variants. Use per-block breakpoints to be
    portable.
  - Routing changes (provider preference, fallback) silently kill the cache.

**Verdict**: our OpenAI-compat path through OpenRouter for Anthropic models requires
explicit `cache_control` to be stamped in the OpenAI-shaped messages we send.
`internal/provider/openai/openai.go` references `CachePolicy` but we have not verified
it actually emits `cache_control` blocks when the upstream is Anthropic via OpenRouter.
Worth a wire test.

Source: <https://openrouter.ai/docs/guides/best-practices/prompt-caching>.

## 4. oMLX (vidar:1235)

- **Cache type**: two-tier KV cache (RAM hot + SSD cold), paged prefix sharing
  inspired by vLLM, persisted to disk in safetensors. Designed for *exactly* the
  cross-HTTP-request reuse problem agent harnesses hit.
- **Opt-in**: **automatic**. No request fields, no headers. Server matches incoming
  prompt tokens against its trie/page index and reuses the longest common prefix.
- **Telemetry**: not standardised. oMLX is OpenAI- and Anthropic-compatible, so the
  response shape is OpenAI's; whether it populates `prompt_tokens_details.cached_tokens`
  is **unverified** — needs an empirical probe (send same prompt twice, diff the usage
  block and the wall-clock TTFT). Server logs / menubar admin dashboard show cache
  hit info but that is out-of-band.
- **Constraints**: prefix must match byte-for-byte (tokenizer-stable). Model must
  remain loaded; eviction is LRU at the page level. SSD persistence means cache
  *can* survive model unload/reload, unlike base mlx-lm.
- **Traps**:
  - Tools/system reordering changes the prefix.
  - Inherits mlx-lm's prefix-cache limitation: pure full-attention models work; SWA /
    Mamba / hybrid-attention models silently recompute (issue ml-explore/mlx-lm#980).
    Qwen3 dense, Llama-class: fine. Gemma3, Qwen3-Next, and similar hybrids: needs
    verification per model.

**Verdict**: oMLX is the only local server in our list with first-class persistent
prefix caching, and it requires zero wire-level effort. The action item is purely
*observational*: add a benchmarking probe that issues an identical request twice and
records the TTFT delta plus any cached-token telemetry the server returns. We should
also confirm per model whether prefix sharing actually fires (it won't for hybrids).

Sources: <https://omlx.ai/>, <https://github.com/jundot/omlx>,
<https://github.com/ml-explore/mlx/discussions/3203>.

## 5. LM Studio

- **Cache type**: KV cache reuse via the underlying llama.cpp / MLX backend.
  Unified KV cache (since 0.4.0) is on by default.
- **Opt-in**: automatic; no wire field. Continuous batching with parallel slots is
  on by default.
- **Telemetry**: none in the OpenAI-compat response. TTFT is the only signal.
- **Traps**:
  - The Anthropic-compat endpoint (`/v1/messages`) has been observed to fully
    reprocess prompts where `/v1/chat/completions` reuses correctly — prefer
    OpenAI-compat surface (lmstudio-bug-tracker#1563-class issues).
  - Some models (Qwen3.5-A3B MoE family, hybrid-attention models) silently disable
    cache reuse on the llama.cpp backend.

Source: <https://lmstudio.ai/blog/0.4.0>.

## 6. vLLM

- **Cache type**: Automatic Prefix Caching (APC), trie-hashed KV blocks.
- **Opt-in**: server-side flag `--enable-prefix-caching` (or `enable_prefix_caching=True`).
  No request field.
- **Telemetry**: vLLM's OpenAI-compat layer populates
  `usage.prompt_tokens_details.cached_tokens` when APC fires.
- **Constraints**: only meaningful during prefill; no effect on decode time. Hash
  algorithm is `builtin` by default; switch to `sha256` for collision resistance.
- **Traps**: APC silently does nothing if not enabled at server start.

Source: <https://docs.vllm.ai/en/latest/features/automatic_prefix_caching/>.

## 7. llama.cpp / llama-server

- **Cache type**: in-process KV cache plus optional KV-shifting cache reuse for
  partially-matching prefixes.
- **Opt-in**: per-request `cache_prompt: true` (default true). Server flag
  `--cache-reuse N` sets the minimum chunk size for KV-shift reuse (0 disables).
  Recent (v1.70+, Oct 2025 PR #16391) host-memory prompt cache adds disk-backed
  persistence.
- **Telemetry**: none in OpenAI-compat response. Server logs only.
- **Traps**: known regressions where `--cache-reuse` stops firing for specific
  models (Qwen3-Next #18497, Gemma 4 #21468 unless `-fa` + `--swa-full`).
  `--prompt-cache` flag is **CLI-only**, not exposed via llama-server.

Source: <https://github.com/ggml-org/llama.cpp/discussions/13606>.

## 8. Ollama

- **Cache type**: in-process KV cache reuse on shared prefixes; sliding-context shift.
- **Opt-in**: automatic. `keep_alive` (request field, e.g. `"60m"` or `-1`) keeps the
  model resident so the cache survives. `num_keep` pins a prefix during context shift.
- **Telemetry**: none in the response. `prompt_eval_count` vs `prompt_eval_duration`
  delta is the only proxy (a hit shows much smaller eval time).
- **Traps**: any change to `num_ctx`, quantisation, or model parameters triggers
  reload and cache wipe. Default 5min `keep_alive` discards cache between idle gaps.
  Some Gemma-3-class models bypass cache (ollama#11365).

Source: <https://docs.ollama.com/faq>.

## 9. Google Gemini (gemini harness, native API)

- **Cache type**: two flavours.
  - **Implicit**: automatic on Gemini 2.5+. Min 2048 input tokens (2.5 Flash, 2.5 Pro).
  - **Explicit**: pre-create a `cachedContents` resource via the Caches API, then
    reference its `name` in the `cachedContent` field of `generateContent`.
- **Opt-in**:
  - Implicit: nothing; just keep prefixes stable.
  - Explicit: `POST /v1beta/cachedContents` with `model`, `contents`, optional `ttl`.
    Then `generateContent` with `"cachedContent": "cachedContents/<id>"`.
- **Telemetry**: `usageMetadata.cachedContentTokenCount` (also called
  `cached_content_token_count` in some SDKs).
- **Discount**: 90% on 2.5+ models, 75% on 2.0.
- **Traps**: implicit cache TTL is short (~3-5 min) and not configurable; explicit
  cache has minimum sizes per model (commonly 4096+ tokens) and incurs storage cost.

Sources: <https://ai.google.dev/gemini-api/docs/caching>,
<https://developers.googleblog.com/gemini-2-5-models-now-support-implicit-caching/>.

---

## Cross-cutting notes

- All providers above key the cache on a byte-stable prefix. Anywhere we inject a
  timestamp, ephemeral run-id, or non-deterministic tool ordering into the system
  prompt or tool list we are paying full prefill on every turn.
- Local servers (oMLX, LM Studio, llama-server, Ollama, vLLM) all do prefix reuse
  *server-side and automatically* once configured — nothing to send on the wire — but
  expose **zero standardised telemetry** in their OpenAI-compat responses. Hit rate
  has to be measured via TTFT or server logs.
- Anthropic and Gemini are the only providers in this list that meaningfully tax the
  client (Anthropic: place breakpoints; Gemini explicit: create a cachedContents).
  Everything else is either automatic (OpenAI, OpenRouter-non-Anthropic, Gemini
  implicit, all local) or a server-side flag.
