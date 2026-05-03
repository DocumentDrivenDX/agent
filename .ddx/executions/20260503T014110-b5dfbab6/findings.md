# Investigation: bench/largest-eigenval — Sonnet PASS vs Vidar reasoning stall

Bead: `fizeau-c0c39e55`
Date: 2026-05-02
Runs:
- PASS — `smoke-largest-eigenval-20260503T002937Z` (Sonnet 4.6 via openrouter), reward 1.0 (with `AgentTimeoutError` reported by harness; reward written before timeout fired)
- FAIL — `smoke-largest-eigenval-20260503T004517Z` (Vidar / Qwen3.6-27B-MLX-8bit via omlx), reward 0.0, reasoning stall

## 1. Confirmed observations

### Sonnet 4.6 — reward 1.0
Harbor run dir (host, not in worktree):
`benchmark-results/harbor-jobs/smoke-largest-eigenval-20260503T002937Z/`

Reported by the bead description: reward 1.0. The run also surfaced an
`AgentTimeoutError` from the harness, but the verifier reward was
already on disk by the time the timeout fired — i.e. the agent reached a
correct solution before the wall clock cap, and the late timeout did not
invalidate the pass. This is consistent with prior Sonnet smoke runs in
this batch where the per-job wall-clock ceiling is tighter than the
agent's own internal step budget.

### Vidar / Qwen3.6-27B-MLX-8bit — reward 0.0
Harbor run dir (host, not in worktree):
`benchmark-results/harbor-jobs/smoke-largest-eigenval-20260503T004517Z/`

Excerpted from the bead description:

```
reasoning stall: model produced only reasoning tokens past stall timeout
tokens: 0 in / 0 out (didn't even start)
```

This is the **second 0/0 sub-shape** observed in the smoke series (the
first was `log-summary-date-ranges` in `fizeau-ddd80cec`). The MLX
chat-completions stream opened but produced zero usable content,
tool-call, or reasoning deltas inside the 5-minute
`DefaultReasoningStallTimeout`. As before, the harness classifies this as
a reasoning stall via the `nonReasoningSeen == false` branch in
`internal/core/stream_consume.go:160-194` — the absence of *any* delta
is treated identically to "only reasoning deltas" by the guard.

Updated table of stall sub-shapes across the smoke series:

| Task                       | Reasoning tokens at abort |
|----------------------------|---------------------------|
| break-filter-js-from-html  | 141                       |
| sqlite-db-truncate         | 4496                      |
| sanitize-git-repo          | 355                       |
| log-summary-date-ranges    | 0                         |
| **largest-eigenval**       | **0**                     |

## 2. Hypothesis: validated — pattern is profile-level, 0/0 sub-shape recurring

The bead's hypothesis ("5th task showing this same pattern; Vidar's
Qwen3.6-27B-MLX-8bit systematically fails with reasoning stalls") is now
supported by five independent task-level confirmations in the same
smoke regime, with two of those five exhibiting the 0/0 (no-delta)
variant. The fact that the 0/0 sub-shape now repeats — rather than
appearing once and never again — argues against a one-off MLX warm-up
race and toward a reproducible failure mode of the Qwen3.6-27B-MLX-8bit
+ reasoning=medium configuration on this task class.

The bead's note that prior Vidar passes (`nginx-request-logging`,
`openssl-selfsigned-cert`, `git-multibranch`) were short/simple is
consistent with the "stall window vs. throughput" framing established
in `fizeau-ff3150d4`/`fizeau-cfe8c5af`/`fizeau-efed53f4`/`fizeau-ddd80cec`:
short tasks complete inside the budget; longer / mathematically dense
prompts do not.

`largest-eigenval` is categorized `mathematics / medium` in
`scripts/beadbench/external/termbench-full.json:209-213`, matching the
profile of prior medium-difficulty failures.

## 3. Configuration in scope

Identical to `fizeau-ff3150d4`, `fizeau-cfe8c5af`, `fizeau-efed53f4`,
and `fizeau-ddd80cec`:

- `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml` →
  `sampling.reasoning: medium` → 8192-token thinking budget
  (`internal/reasoning/reasoning.go`,
  `PortableBudgets[ReasoningMedium] = 8192`).
- Wire format: Qwen `enable_thinking: true` / `thinking_budget: 8192`
  (`internal/provider/openai/openai.go`).
- The MLX server treats `thinking_budget` as advisory; the harness
  5-minute reasoning-stall guard fires before the budget runs out at
  this model's throughput.

## 4. Why this is a divergence, not a flake

- Sonnet ran the same `largest-eigenval` task to reward 1.0 in the
  adjacent smoke batch — the task is achievable in budget on a frontier
  model with the same harness/prompt.
- The Vidar arm did not fail on verifier output; it failed before
  producing any agent step (no trajectory, no tool calls, no tokens at
  all on this run).
- This is the **fifth independent confirmation** of the
  Qwen3.6-27B-MLX-8bit reasoning-stall signature in the same smoke
  regime (`break-filter-js-from-html`, `sqlite-db-truncate`,
  `sanitize-git-repo`, `log-summary-date-ranges`, `largest-eigenval`).
  The pattern is profile-level, not task-specific. The 0/0 variant has
  now reproduced on a second task, raising it from "novel sub-shape" to
  a reproducible failure mode of this configuration.

## 5. Recommendations (no code changed in this bead)

Recommendations from the prior four beads stand. The fifth confirmation
sharpens them:

1. **Lower reasoning effort for vidar smoke (now blocking)** — flip
   `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml` `sampling.reasoning`
   from `medium` (8192) → `low` (2048). This recommendation has been
   open since `fizeau-efed53f4` and was marked overdue in
   `fizeau-ddd80cec`. With a fifth confirmation and a recurring 0/0
   sub-shape, additional smoke evidence is no longer informative until
   the budget is lowered. File / progress the dedicated bead and
   re-run all five failing tasks under the lower budget.
2. **First-token latency telemetry in harbor results** — the second
   appearance of the 0/0 sub-shape makes "model never spoke" a
   reproducible class, not a one-off. Surface stream-open timestamp,
   first-byte latency, and first-delta latency in the harbor result so
   the 0/0 stall can be split between (a) MLX cold-start / chat-template
   slow path vs. (b) suppressed deltas while thinking. Carried over
   from `fizeau-ddd80cec` recommendation #2 — now justified by recurrence.
3. **Do not raise the 5-minute stall timeout** — masks the symptom.
4. **Defer to AR-2026-04-26**
   (`docs/research/AR-2026-04-26-agent-vs-pi-omlx-vidar-qwen36.md`) on
   agent-vs-pi parity for OMLX/Vidar/Qwen3.6-27B before drawing routing
   conclusions.
5. **Sonnet `AgentTimeoutError`-after-pass** is benign on this run
   (reward written first) but worth a follow-up: if the harness can
   distinguish "timeout after reward written" from "timeout before
   reward written", the smoke summary should not flag the former as an
   error. Out of scope for this bead.

## 6. Evidence pointers

- `internal/core/stream_consume.go:21` — `DefaultReasoningStallTimeout = 300s`
- `internal/core/stream_consume.go:160-194` — stall detection branch
- `internal/core/errors.go` — `ErrReasoningStall`
- `internal/reasoning/reasoning.go` — `PortableBudgets`
- `internal/provider/openai/openai.go` — Qwen thinking wire format
- `internal/provider/omlx/omlx.go` — `ProtocolCapabilities` with
  `ThinkingFormat: ThinkingWireFormatQwen`, `StrictThinkingModelMatch: true`
- `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml` —
  `sampling.reasoning: medium`
- `scripts/beadbench/external/termbench-full.json:209-213` — task
  classification (`mathematics / medium`)
- Prior identical divergences:
  - bead `fizeau-ff3150d4`, findings at
    `.ddx/executions/20260502T231952-3d7f445d/findings.md`
    (break-filter-js-from-html)
  - bead `fizeau-cfe8c5af`, findings at
    `.ddx/executions/20260503T001340-2daf470c/findings.md`
    (sqlite-db-truncate)
  - bead `fizeau-efed53f4`, findings at
    `.ddx/executions/20260503T002325-c7a9fe81/findings.md`
    (sanitize-git-repo)
  - bead `fizeau-ddd80cec`, findings at
    `.ddx/executions/20260503T002945-dfec88f5/findings.md`
    (log-summary-date-ranges)
- Harbor run dirs (host, not in worktree):
  - `benchmark-results/harbor-jobs/smoke-largest-eigenval-20260503T002937Z/`
  - `benchmark-results/harbor-jobs/smoke-largest-eigenval-20260503T004517Z/`
