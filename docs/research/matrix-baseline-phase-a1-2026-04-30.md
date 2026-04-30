---
ddx:
  id: matrix-baseline-phase-a1-2026-04-30
  bead: agent-b0500f6a
  depends_on:
    - SD-010
    - agent-d7d2e4dd
---

# Phase A.1 Matrix Baseline — Blocked Preflight

Status: blocked on live execution prerequisites.

This memo records the Phase A.1 attempt state for bead `agent-b0500f6a`. The
matrix runner, aggregator, adapters, anchor profile, and cost guardrails are
implemented, but the live paid Phase A.1 matrix was not executed in this
environment.

## Caveat: same-model-different-harness comparison

Cells in this matrix that share a model column and differ only by harness row
are not a clean control of model capability. Each harness ships its own system
prompt, tool schema, retry policy, reasoning effort, context compaction
strategy, and default sampling. The numbers below compare (harness scaffolding
+ policy) over a shared model API, not pure harness skill, and not pure model
skill. Differences in scaffolding, prompt template, tool surface, and turn
budget account for an unknown share of any observed delta. See SD-010 §2 D4
(telemetry schema), §5 (failure taxonomy), and §7 for the full obligations.

## Preflight

Commands executed on 2026-04-30:

```sh
if [ -n "$OPENAI_API_KEY" ]; then echo OPENAI_API_KEY=set; else echo OPENAI_API_KEY=unset; fi
for bin in harbor pi opencode; do if command -v "$bin" >/dev/null 2>&1; then echo "$bin=$(command -v "$bin")"; else echo "$bin=missing"; fi; done
```

Observed result:

```text
OPENAI_API_KEY=unset
harbor=missing
pi=/home/linuxbrew/.linuxbrew/bin/pi
opencode=missing
```

Because the anchor profile `gpt-5-3-mini` requires `OPENAI_API_KEY`, a live run
would fail before producing acceptance-grade graded cells. Harbor and opencode
are also missing from this environment.

## Runner Verification

The no-API runner path was verified before this memo:

```sh
ddx-agent-bench matrix \
  --work-dir . \
  --harnesses=cost_probe \
  --profiles=smoke \
  --reps=2 \
  --per-run-budget-usd=0.000001 \
  --out <tmp>

ddx-agent-bench matrix-aggregate <tmp>
```

That synthetic check produced six persisted `budget_halted` runs and a
`costs.json` with `per_run_cap_usd`. It proves guardrail behavior, not model or
harness quality.

Repository verification also passed:

```sh
go test ./...
python3 -m unittest \
  scripts.benchmark.harness_adapters.test_base_calibration \
  scripts.benchmark.harness_adapters.test_pi \
  scripts.benchmark.harness_adapters.test_opencode
```

## Follow-Up Work

Root-cause follow-up work is tracked under epic `agent-d7d2e4dd`:

- `agent-73f90363` — provision the Phase A.1 live matrix prerequisites.
- `agent-5b6f5872` — run the live Phase A.1 matrix after prerequisites.

No Phase A.1 reward table, per-task pass count, or observed paid cost
reconciliation is published from this environment because doing so would
fabricate benchmark evidence.
