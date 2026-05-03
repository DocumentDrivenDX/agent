# Investigation: bench/sanitize-git-repo — Sonnet PASS vs Vidar reasoning stall

Bead: `fizeau-efed53f4`
Date: 2026-05-02
Runs:
- PASS — `smoke-sanitize-git-repo-20260502T234645Z` (Sonnet 4.6 via openrouter), reward 1.0
- FAIL — `smoke-sanitize-git-repo-20260503T000704Z` (Vidar / Qwen3.6-27B-MLX-8bit via omlx), reward 0.0

## 1. Confirmed observations

### Sonnet 4.6 — reward 1.0
Harbor run dir (host, not in worktree):
`benchmark-results/harbor-jobs/smoke-sanitize-git-repo-20260502T234645Z/`

Reported by the bead description: status success, reward 1.0.

### Vidar / Qwen3.6-27B-MLX-8bit — reward 0.0
Harbor run dir (host, not in worktree):
`benchmark-results/harbor-jobs/smoke-sanitize-git-repo-20260503T000704Z/`

Excerpted from the bead's failure log:

```
reasoning stall: model produced only reasoning tokens past stall timeout
  (model=Qwen3.6-27B-MLX-8bit, timeout=5m0s)
tokens: 2856 in / 355 out
```

The 355 output tokens — all classified as `reasoning_content` over the Qwen
wire format — is notably smaller than the prior two stalls (4496 tokens for
sqlite-db-truncate, 141 for break-filter-js-from-html). The wall-clock to
abort = 5 minutes (default `DefaultReasoningStallTimeout`). The low output
count combined with the 5-minute wall clock implies the MLX server was
emitting reasoning deltas slowly (≈1.2 tok/s) while never transitioning to
content or tool-call deltas — same stream-shape as the prior two failures,
just at a lower throughput. Harbor surfaces the abort as
`NonZeroAgentExitCodeError`.

## 2. Hypothesis: validated

The bead's hypothesis ("same pattern as break-filter-js-from-html and
sqlite-db-truncate — Qwen3.6-27B-MLX-8bit stalls in extended thinking mode;
systemic across medium-difficulty tasks") matches the evidence.

The stall is fired by `internal/core/stream_consume.go:173`:

```go
if stallTimeout > 0 && time.Since(reasoningStallStart) > stallTimeout {
    stallErr := newReasoningStallError(...)
    ...
    return resp, stallErr
}
```

Reachable only while `nonReasoningSeen == false` — the provider stream
produced exclusively `ReasoningContent` deltas, never a content delta or
tool-call delta. Default `DefaultReasoningStallTimeout = 300 * time.Second`
(`internal/core/stream_consume.go:21`) matches the `timeout=5m0s` in the
error message. The error type is `ErrReasoningStall`
(`internal/core/errors.go:33`).

## 3. Configuration in scope

Identical to `fizeau-ff3150d4` and `fizeau-cfe8c5af`:

- `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml:21` →
  `sampling.reasoning: medium` → 8192-token thinking budget
  (`internal/reasoning/reasoning.go:30`,
  `PortableBudgets[ReasoningMedium] = 8192`).
- Wire format: Qwen `enable_thinking: true` / `thinking_budget: 8192`
  (`internal/provider/openai/openai.go:386-390`).
- The MLX server treats `thinking_budget` as advisory; in practice the
  thinking stream runs until either it self-terminates or the harness
  trips the 5-minute stall first. On this task the latter happened.

## 4. Why this is a divergence, not a flake

- Sonnet ran the same sanitize-git-repo task to PASS in the same smoke
  batch — task is achievable in budget on a frontier model with the same
  harness/prompt.
- Vidar arm did not fail on a verifier-output mismatch; it failed before
  producing any agent step (no trajectory steps, no tool calls, only 355
  reasoning tokens).
- This is the **third independent confirmation** of the
  Qwen3.6-27B-MLX-8bit reasoning-stall signature in the same smoke regime
  (now: `break-filter-js-from-html`, `sqlite-db-truncate`,
  `sanitize-git-repo`). The pattern is profile-level, not task-specific.
- The low token count on this run (355 vs 141 vs 4496) further argues
  against per-task variance: the failure mode is "never emits a tool call",
  irrespective of how much internal monologue the model produces first.

## 5. Recommendations (no code changed in this bead)

The recommendations from `fizeau-ff3150d4` and `fizeau-cfe8c5af` stand. With
a third confirming data point, recommendation #1 is now the highest-priority
follow-up and should be filed as its own actionable bead rather than
deferred:

1. **Lower reasoning effort for vidar smoke (top priority)** —
   `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml` `sampling.reasoning`
   from `medium` (8192) → `low` (2048). With three stalls in two batches
   this is the most likely-yield change. Re-run all three failing tasks
   (`break-filter-js-from-html`, `sqlite-db-truncate`, `sanitize-git-repo`)
   to confirm the tighter budget pushes the model into the tool loop
   within the 5-minute stall window.
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
- Prior identical divergences:
  - bead `fizeau-ff3150d4`, findings at
    `.ddx/executions/20260502T231952-3d7f445d/findings.md`
    (break-filter-js-from-html)
  - bead `fizeau-cfe8c5af`, findings at
    `.ddx/executions/20260503T001340-2daf470c/findings.md`
    (sqlite-db-truncate)
- Harbor run dirs (host, not in worktree):
  - `benchmark-results/harbor-jobs/smoke-sanitize-git-repo-20260502T234645Z/`
  - `benchmark-results/harbor-jobs/smoke-sanitize-git-repo-20260503T000704Z/`
