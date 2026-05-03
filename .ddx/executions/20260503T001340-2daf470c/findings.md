# Investigation: bench/sqlite-db-truncate — Sonnet PASS vs Vidar reasoning stall

Bead: `fizeau-cfe8c5af`
Date: 2026-05-02
Runs:
- PASS — `smoke-sqlite-db-truncate-20260502T231911Z` (Sonnet 4.6 via openrouter), reward 1.0
- FAIL — `smoke-sqlite-db-truncate-20260502T232057Z` (Vidar / Qwen3.6-27B-MLX-8bit via omlx), reward 0.0

## 1. Confirmed observations

### Sonnet 4.6 — reward 1.0
Harbor run dir (host, not in worktree):
`benchmark-results/harbor-jobs/smoke-sqlite-db-truncate-20260502T231911Z/`

Reported by the bead description: status success, reward 1.0.

### Vidar / Qwen3.6-27B-MLX-8bit — reward 0.0
Harbor run dir (host, not in worktree):
`benchmark-results/harbor-jobs/smoke-sqlite-db-truncate-20260502T232057Z/`

`agent/fiz.txt` tail (excerpted in bead):

```
2026/05/02 23:32:24 WARN reasoning loop detected: aborting stream
  err="agent: reasoning stall: model produced only reasoning tokens past stall timeout
       (model=Qwen3.6-27B-MLX-8bit, timeout=5m0s)"
{
  "status": "failed",
  "error": "agent: reasoning stall: model produced only reasoning tokens past stall timeout",
  "tokens": { "input": 16150, "output": 4496, "total": 20646 }
}
```

The 4496 output tokens are consistent with thinking tokens classified as
`reasoning_content` (not user-visible content) over the Qwen wire format.
Wall-clock to abort = 5 minutes (default `DefaultReasoningStallTimeout`).
Harbor surfaces this as `NonZeroAgentExitCodeError`.

## 2. Hypothesis: validated

The bead's hypothesis ("same reasoning stall pattern as
break-filter-js-from-html — Qwen3.6-27B-MLX-8bit entered extended thinking
mode without producing tool calls, triggering the 5-minute stall timeout")
matches the evidence exactly. Compared to the previous occurrence
(bead `fizeau-ff3150d4`, doc `.ddx/executions/20260502T231952-3d7f445d/findings.md`),
the only material difference is that this run produced **more** thinking
tokens before the stall fired (4496 vs 141). That is consistent with the
SQLite debugging task being more textually rich than the XSS bypass — the
model has more to chew on internally — but the failure mode is identical:
zero tool-call steps, five-minute wall-clock timeout, no agent action.

The stall is fired by `internal/core/stream_consume.go:173`:

```go
if stallTimeout > 0 && time.Since(reasoningStallStart) > stallTimeout {
    stallErr := newReasoningStallError(...)
    ...
    return resp, stallErr
}
```

Reachable only while `nonReasoningSeen == false` — provider stream produced
exclusively `ReasoningContent` deltas, never a content delta or tool-call
delta. Default `DefaultReasoningStallTimeout = 300 * time.Second`
(`internal/core/stream_consume.go:21`) matches the `timeout=5m0s` in the
error message. The error type is `ErrReasoningStall`
(`internal/core/errors.go:33`).

## 3. Configuration in scope

Identical to `fizeau-ff3150d4`:

- `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml:21` →
  `sampling.reasoning: medium` → 8192-token thinking budget
  (`internal/reasoning/reasoning.go:30`,
  `PortableBudgets[ReasoningMedium] = 8192`).
- Wire format: Qwen `enable_thinking: true` /  `thinking_budget: 8192`
  (`internal/provider/openai/openai.go:386-390`).
- The MLX server treats `thinking_budget` as advisory; in practice the
  thinking stream runs until either it self-terminates or the harness
  trips the 5-minute stall first. On this task the latter happened.

## 4. Why this is a divergence, not a flake

- Sonnet ran the same SQLite-truncation task to PASS in the same smoke
  batch — task is achievable in budget on a frontier model with the same
  harness/prompt.
- Vidar arm did not fail on a verifier-output mismatch; it failed before
  producing any agent step (no trajectory steps, no tool calls).
- Two independent smoke tasks now show the same Qwen3.6-27B-MLX-8bit
  failure signature in one batch (`break-filter-js-from-html` and
  `sqlite-db-truncate`). This is a model/profile-level pattern, not
  task-specific.
- It is consistent with Qwen3 thinking models on multi-step debugging
  tasks: open-ended exploratory reasoning without an external forcing
  function to commit to action.

## 5. Recommendations (no code changed in this bead)

The recommendations from `fizeau-ff3150d4` apply as-is. With a second
confirming data point, the priority of #1 increases:

1. **Lower reasoning effort for vidar smoke** —
   `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml` `sampling.reasoning`
   from `medium` (8192) → `low` (2048). With two stalls in one batch this
   is now the most likely-yield change. Re-run both `break-filter-js-from-html`
   and `sqlite-db-truncate` to confirm the tighter budget pushes the model
   into the tool loop within 5 minutes.
2. **Track stall-rate as a first-class benchmark metric** — preserve
   `reasoning_tail` and `model` payload from
   `internal/core/stream_consume.go:181-187` into the harbor result so
   future divergences surface without log archaeology.
3. **Do not raise the 5-minute stall timeout** — masks the symptom.
4. **Defer to AR-2026-04-26** (`docs/research/AR-2026-04-26-agent-vs-pi-omlx-vidar-qwen36.md`)
   on agent-vs-pi parity for OMLX/Vidar/Qwen3.6-27B before drawing routing
   conclusions.

## 6. Evidence pointers

- `internal/core/stream_consume.go:21` — `DefaultReasoningStallTimeout = 300s`
- `internal/core/stream_consume.go:160-194` — stall detection branch
- `internal/core/errors.go:33` — `ErrReasoningStall`
- `internal/reasoning/reasoning.go:27-30` — `PortableBudgets`
- `internal/provider/openai/openai.go:340-398` — Qwen thinking wire format
- `internal/provider/omlx/omlx.go:40-50` — `ProtocolCapabilities` with
  `ThinkingFormat: ThinkingWireFormatQwen`, `StrictThinkingModelMatch: true`
- `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml:21` —
  `sampling.reasoning: medium`
- Prior identical divergence: bead `fizeau-ff3150d4`, findings at
  `.ddx/executions/20260502T231952-3d7f445d/findings.md`
- Harbor run dirs (host, not in worktree):
  - `benchmark-results/harbor-jobs/smoke-sqlite-db-truncate-20260502T231911Z/`
  - `benchmark-results/harbor-jobs/smoke-sqlite-db-truncate-20260502T232057Z/`
