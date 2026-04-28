# AR-2026-04-28 — Agent vs Pi on openrouter qwen/qwen3.6-plus

Status: complete (one paired run; both arms surfaced bugs that block clean comparison).

Governing artifact: [`agent-f43d1ed2`](../../.ddx/beads.jsonl) (recurring parity tracker, depends on initial measurement `agent-3663e287`).

This is the first row in the agent-vs-pi parity tracker ([agent-vs-pi-parity.md](agent-vs-pi-parity.md)). The methodology is established; the actual numbers are noisy because both arms hit infrastructure bugs we hadn't surfaced before.

## 1. Methodology

Paired design. One bead, two arms, same backing:

- **agent-openrouter-qwen36plus** — native agent harness, provider `openrouter`, model `qwen/qwen3.6-plus`.
- **pi-openrouter-qwen36plus** — pi harness, provider `openrouter`, model `qwen/qwen3.6-plus`.

Task: `agent-beadbench-preflight` (bead `agent-37aeb88e`). Verify gate: `go test ./...`.

Reviewer pin: codex / gpt-5.5 (per the methodology in
[AR-2026-04-26-agent-vs-pi-omlx-vidar-qwen36](AR-2026-04-26-agent-vs-pi-omlx-vidar-qwen36.md)).

Sample size: N=1. Methodology validation only — too small for confidence intervals.

## 2. Provider configuration evidence

`pi --list-models` (excerpt, 2026-04-28):

```
openrouter         qwen/qwen3.6-plus                             1M       65.5K    yes       yes
```

Pi reaches the same backing the agent arm uses. Quality of comparison is therefore harness-isolated when both arms run.

Both providers configured in `~/.config/agent/config.yaml` and the corresponding `pi`-side config (managed by pi, opaque to us per the user directive "just consume what pi tells us").

## 3. Per-arm outcomes

Run dir: `benchmark-results/beadbench/run-20260428T032010Z-1595674/`.

| Arm | Status | Duration | Cost | Tokens | Verify | Notes |
|-----|--------|----------|------|--------|--------|-------|
| agent-openrouter-qwen36plus | execute=success | 366.4 s | $0.480 | 1,399,475 | **fail** | Model produced a patch; verify gate failed on test-config-isolation bug (agent-27806ad5) — `go test ./...` from the bead's pre-lucebox verify-worktree rejected the user's live config which contains `type: lucebox`. |
| pi-openrouter-qwen36plus | execute=execution_failed | 21.4 s | n/a | n/a | skipped | `panic: send on closed channel` in `internal/harnesses/pi/runner.go:338` (`mirroredEvents`). Pi never reached the model. Filed as `agent-195bb183`. |

## 4. Aggregate metrics + winner

Two divergent failure modes that don't cleanly compare:

- **Agent verified-pass: 0/1.** But the model + harness produced a patch and exited cleanly. The verify-gate failure is a test-isolation defect downstream of the harness.
- **Pi verified-pass: 0/1.** Pi crashed before model contact. The harness wrapper itself panicked.

For the parity-tracker schema, both rows record `verified_pass = 0`. **Δ = 0 pp.** Cost ratio is `n/a` because pi didn't run long enough to attribute spend.

**Winner declaration: tie, both arms blocked.** The match-criterion clock does NOT advance; the methodology is validated in the procedural sense (paired arms run, captured artifacts compared) but not in the data sense (no usable per-arm signal yet).

## 5. Top-3 gaps

These are the immediate blockers for the next paired run to be informative:

1. **Pi-runner panic (`agent-195bb183`).** `send on closed channel` in `mirroredEvents`. Likely a context-cancellation race; needs a select-on-ctx.Done() or a defensive `chan` guard at runner.go:330-340. Pi can't participate in any parity measurement until this lands.

2. **Test-config isolation (`agent-27806ad5`).** `service_*_test.go` calls `agent.New()` with default options, which loads the live user config. Bead-verify-worktrees at older base revisions reject any provider type the older code didn't know about — making the verify gate fail through no fault of the bead-completion path. Fix: tests use `t.TempDir()` for `XDG_CONFIG_HOME` or pass an explicit ServiceConfig stub.

3. **Verify-gate signal noise.** Today's run highlighted that the agent-beadbench-preflight task's `go test ./...` verify gate is sensitive to the agent's _ambient environment_ rather than just the bead's correctness. Either narrow the verify gate (target packages the bead's diff actually touches) or factor out global-config dependencies in the test set so the gate measures bead correctness, not config-vs-version compatibility.

Not blocking but worth noting: agent-side run consumed 1.4M tokens and $0.48 for what previously took 915k tokens / $0.32 in the Tier-3 cross-provider sweep earlier today. Likely model non-determinism on cloud Qwen-plus, but worth eyeing whether bead-prompt churn between runs accidentally inflated context.
