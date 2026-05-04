# Parent close evidence: agent-d7d2e4dd

Bead `agent-d7d2e4dd` is a parent epic for Phase A.1 live matrix blockers.
Its acceptance requires all child blocker beads to be closed and a subsequent
live Phase A.1 matrix run to execute with `ddx-agent`, `pi`, and `opencode`
against the anchor profile without prerequisite failures.

## Child status

- `agent-73f90363` (`Provision Phase A.1 live matrix prerequisites`): closed
  at commit `d252fdba44fef4f6f05c1ea3f4afd84bc53674fe`.
- `agent-5b6f5872` (`Run Phase A.1 live matrix after prerequisites`): closed
  at commit `ee52c22f08c46a2a021568e9c86c0697444047b0`.
- `fizeau-01248b3d` (`Provide Terminal-Bench canary task bundle for Phase A.1
  matrix`): closed; this was the root-cause follow-up cited by the live matrix
  report for non-graded terminal statuses.

## Live matrix evidence

The live child published:

- `benchmark-results/matrix-20260504T044909Z/matrix.json`
- `benchmark-results/matrix-20260504T044909Z/matrix.md`
- `benchmark-results/matrix-20260504T044909Z/costs.json`
- `docs/research/matrix-baseline-phase-a1-2026-05-04.md`

That report records the shipped runner and aggregator commands with
`--harnesses=ddx-agent,pi,opencode`, `--profiles=gpt-5-mini`, and the anchor
subset. All 27 planned runs reached terminal `final_status`; the later
Terminal-Bench task-bundle failure was tracked separately as `fizeau-01248b3d`
and is not one of this parent epic's original prerequisite failures.

## Local verification

Commands run from the repository root:

```sh
test -n "$OPENROUTER_API_KEY" &&
  command -v harbor pi opencode >/dev/null &&
  ddx-agent-bench profiles list --work-dir . | rg -q '(^|[[:space:]])gpt-5-mini([[:space:]]|$)' &&
  ddx bead show agent-73f90363 | rg -q 'Status:[[:space:]]+closed' &&
  ddx bead show agent-5b6f5872 | rg -q 'Status:[[:space:]]+closed' &&
  ddx bead show fizeau-01248b3d | rg -q 'Status:[[:space:]]+closed'

go test ./...
```

Both commands exited 0 on 2026-05-04.
