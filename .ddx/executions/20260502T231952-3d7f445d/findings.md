# Investigation: bench/break-filter-js-from-html — Sonnet PASS vs Vidar reasoning stall

Bead: `fizeau-ff3150d4`
Date: 2026-05-02
Runs:
- PASS — `smoke-break-filter-js-from-html-20260502T225046Z` (Sonnet 4.6 via openrouter)
- FAIL — `smoke-break-filter-js-from-html-20260502T225839Z` (Vidar / Qwen3.6-27B-MLX-8bit via omlx)

## 1. Confirmed observations

### Sonnet 4.6 — reward 1.0
`benchmark-results/harbor-jobs/smoke-break-filter-js-from-html-20260502T225046Z/break-filter-js-from-html__c4Y2DoN/agent/fiz.txt`

- `status: success`, `tokens: 714930 in / 17745 out`, `cost_usd: 2.41`,
  `duration_ms: 413354000000` (~6:53 wall)
- Final answer payload: `<svg><style><script>alert(1)</script></style></svg>` with
  the BeautifulSoup-vs-Chrome-HTML5 parser-discrepancy explanation.
- Reward parsed by harbor: `reward 1.0`, `n_errors: 0`.

### Vidar / Qwen3.6-27B-MLX-8bit — reward 0.0
`benchmark-results/harbor-jobs/smoke-break-filter-js-from-html-20260502T225839Z/break-filter-js-from-html__gbthoFJ/`

- `agent/fiz.txt` — first non-empty line:
  `WARN reasoning loop detected: aborting stream err="agent: reasoning stall: model produced only reasoning tokens past stall timeout (model=Qwen3.6-27B-MLX-8bit, timeout=5m0s)"`
- Final fiz JSON: `status: failed`, `tokens: 2738 in / 141 out`,
  `duration_ms: 352168000000` (~5:52 wall — i.e. 5m stall + ~50s setup/teardown).
- `agent/trajectory.json`: `steps: 0` and
  `final_metrics: {total_prompt_tokens: 0, total_completion_tokens: 0, total_steps: 0}`.
  **The agent never made it to a single tool-call step before the stall fired.**
- harbor: `NonZeroAgentExitCodeError`, no retry (max_retries: 0).

## 2. Hypothesis: validated

The bead's hypothesis ("Qwen3.6-27B-MLX-8bit entered an extended chain-of-thought
reasoning loop without producing any tool calls or output tokens, triggering the
5-minute stall timeout") matches the evidence exactly.

The stall is fired by `internal/core/stream_consume.go:173`:

```go
if stallTimeout > 0 && time.Since(reasoningStallStart) > stallTimeout {
    stallErr := newReasoningStallError(...)
    ...
    return resp, stallErr
}
```

That branch is reachable only while `nonReasoningSeen == false` — i.e. the
provider stream produced exclusively `ReasoningContent` deltas and never a
content delta or tool-call delta. The default timeout is
`DefaultReasoningStallTimeout = 300 * time.Second`
(`internal/core/stream_consume.go:21`), which matches the `timeout=5m0s` in the
error message. The error type is `ErrReasoningStall`
(`internal/core/errors.go:33`).

The 141-token output-side count is consistent with thinking tokens being counted
into `output` while the SSE stream classified every delta as
`reasoning_content` under the Qwen wire format
(`internal/provider/openai/openai.go:386` —
`option.WithJSONSet("enable_thinking", true)`).

## 3. Configuration in scope

`scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml:21` sets `sampling.reasoning: medium`.

`medium` maps to a `thinking_budget` of **8192** tokens via the portable budget
table (`internal/reasoning/reasoning.go:30 PortableBudgets[ReasoningMedium] = 8192`),
sent on the wire as Qwen-format `enable_thinking: true` /
`thinking_budget: 8192` (`internal/provider/openai/openai.go:386-390`).

In practice, for this task the MLX server either:
- streamed thinking tokens slowly enough that 5 minutes elapsed before any
  closing `</think>` (i.e. the budget is advisory and not strictly enforced
  server-side); or
- never reached the budget exhaustion path in the available wall-clock window.

Either way, the upstream effect is identical: zero tool-call steps, stall.

This matches the broader OMLX/Qwen3 evidence already on record:
`docs/research/windows-local-inference-reasoning.md` and
`docs/research/qwen3.6-27b-cross-provider-2026-04-27.md` document that Qwen
thinking-mode controls behave inconsistently across local backends, and
`docs/research/AR-2026-04-26-agent-vs-pi-omlx-vidar-qwen36.md` is the standing
agent-vs-pi parity AR for this exact provider/model.

## 4. Why this is a "divergence" rather than a flake

- Sonnet ran the same XSS bypass task to PASS in ~7 minutes — the task is
  achievable in budget on a frontier model with the same harness/prompt.
- The vidar arm did not fail on a verifier-output mismatch; it failed before
  producing any agent step. So the divergence is *not* about the XSS bypass
  reasoning itself — it's about the model never reaching the tool loop.
- This is consistent with Qwen3 thinking models' behavior on security-flavored
  problems requiring iterative experimentation: open-ended exploratory
  reasoning without an external forcing function to commit to action.

## 5. Recommendations (no code changed in this bead)

1. **Lower reasoning effort for vidar smoke**: change
   `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml` `sampling.reasoning` from
   `medium` (8192) to `low` (2048) for smoke runs. Re-run the same task to
   confirm whether a tighter budget pushes the model out of pure-reasoning into
   the tool loop within 5 minutes.
2. **Track stall-rate as a first-class benchmark metric**: the stall fires with
   `reasoning_tail` and `model` payload (`internal/core/stream_consume.go:181-187`)
   — the harbor result already loses this. A follow-up bead should ensure the
   reasoning-stall event is preserved into the benchmark result so divergences
   like this surface without manual log archaeology.
3. **Do not raise the 5-minute stall timeout** as a remediation. The default
   exists to prevent runaway thinking loops and matches the documented behavior
   in `internal/core/stream_consume.go:17-21`. Raising it masks the symptom.
4. **Defer to the AR-2026-04-26 work** on agent-vs-pi parity for OMLX/Vidar/
   Qwen3.6-27B before drawing routing conclusions from a single divergence.

## 6. Evidence pointers

- `internal/core/stream_consume.go:21` — `DefaultReasoningStallTimeout = 300s`
- `internal/core/stream_consume.go:160-194` — stall detection branch
- `internal/core/errors.go:33` — `ErrReasoningStall`
- `internal/reasoning/reasoning.go:27-30` — `PortableBudgets`
- `internal/provider/openai/openai.go:340-398` — Qwen thinking wire format
- `internal/provider/omlx/omlx.go:40-50` — `ProtocolCapabilities` with
  `ThinkingFormat: ThinkingWireFormatQwen` and `StrictThinkingModelMatch: true`
- `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml:21` — `reasoning: medium`
- Harbor run dirs (host, not in worktree):
  - `benchmark-results/harbor-jobs/smoke-break-filter-js-from-html-20260502T225046Z/`
  - `benchmark-results/harbor-jobs/smoke-break-filter-js-from-html-20260502T225839Z/`
