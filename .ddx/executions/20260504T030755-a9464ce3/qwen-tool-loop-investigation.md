# Investigation: vidar Qwen3.6 low-budget ErrToolCallLoop

Bead: `fizeau-c5654b96`

## Summary

The original `break-filter-js-from-html-20260503T034311Z` Harbor artifacts
named by the bead are not present in this execution worktree. I searched the
tracked tree, `.fizeau/sessions`, `.ddx/attachments`, `.ddx/executions`, and
parent worktree paths for the named `benchmark-results/harbor-jobs/...`
directories and for the repeated command
`mkdir -p /tests && cp /app/filter.py /tests/filter.py`; no matching 11,797
event session JSONL was available here. Therefore I could not prove from the
original session whether the three repeated calls came from a replayed provider
response or from three independent provider turns.

The current codebase already contains the prompt guard that this bead proposes
as a possible mitigation:

- `internal/prompt/presets.go:92-93` says not to repeat the same failed action
  or a tool call that already returned the same result.
- `scripts/benchmark/AGENTS.md:9` says to diagnose and try something different
  instead of repeating the same failing action.

Given the bead's explicit "DO NOT MERGE" constraint, I made no prompt changes.

## Fresh medium-budget comparison

I ran one fresh `break-filter-js-from-html` smoke attempt against the reachable
vidar oMLX endpoint with `FIZEAU_REASONING=medium`:

```sh
timeout 1200s env \
  FIZEAU_BENCH_TASK=break-filter-js-from-html \
  FIZEAU_PROVIDER_NAME=vidar-omlx \
  FIZEAU_PROVIDER=omlx \
  FIZEAU_MODEL=Qwen3.6-27B-MLX-8bit \
  FIZEAU_BASE_URL=http://vidar:1235/v1 \
  FIZEAU_API_KEY_ENV=OMLX_API_KEY \
  FIZEAU_REASONING=medium \
  FIZEAU_BENCH_PRESET=benchmark \
  ./scripts/benchmark/smoke_run.sh
```

Result:

- The run reached Harbor and launched `fiz` in the task container.
- Trial path:
  `benchmark-results/harbor-jobs/smoke-break-filter-js-from-html-20260504T031109Z/break-filter-js-from-html__EshxKtD/`
- The outer 20 minute timeout fired with exit code `124`.
- `agent/fiz.txt` remained `0` bytes.
- No `agent/trajectory.json`, no `verifier/reward.txt`, and no copied
  `agent/sessions/*.jsonl` were produced.
- Harbor wrote `exception.txt`; its traceback is a `KeyboardInterrupt` /
  `CancelledError` caused by the outer timeout while Harbor was still awaiting
  the agent exec stream.

This does not reproduce the low-budget repeated `mkdir/cp` ErrToolCallLoop
shape. It does show that switching this task to medium reasoning does not make
the task pass in this environment; the medium attempt failed to produce any
usable agent/session evidence within 20 minutes.

I removed the leftover Harbor container after the timeout:

```sh
docker rm -f 118908d5a1c2
```

Follow-up process checks showed no remaining `smoke-break-filter-js-from-html`
processes or containers from this attempt.

## Relevant implementation facts

Reasoning budget mapping:

- `internal/reasoning/reasoning.go:27-31` maps `low` to 2048 tokens and
  `medium` to 8192 tokens.
- `internal/provider/openai/openai.go:398-410` serializes Qwen reasoning
  control as `enable_thinking: true` plus `thinking_budget`.
- `scripts/benchmark/profiles/vidar-qwen3-6-27b.yaml:19-23` now pins the vidar
  benchmark profile to `temperature: 0.6`, `reasoning: low`, `top_p: 0.95`,
  `top_k: 20`.

Loop detection:

- `internal/core/loop.go:816-831` aborts after three consecutive identical
  tool-call fingerprints and returns `ErrToolCallLoop`.
- This detector operates after a provider response has been consumed and tool
  results have been appended. It can detect repeated identical calls, but it
  does not currently emit an explicit pivot prompt before aborting.

Reasoning-stall detection:

- `internal/core/stream_consume.go:208-234` adapts the pure-reasoning stall
  deadline based on the configured reasoning budget and emits a structured
  `reasoning.stall` event if the model never produces content or tool calls.

## Conclusion

Do not merge a prompt-only fix from this bead. The current prompt already has
the proposed no-repeat instruction, and the fresh medium-budget rerun did not
demonstrate improvement on `break-filter-js-from-html`.

The next actionable work should be evidence plumbing or loop recovery, not
another static prompt line:

1. Preserve the exact session JSONL for Harbor failures in the benchmark result
   bundle so future investigations can distinguish replayed responses from
   separate identical turns.
2. Add a tested `ErrToolCallLoop` recovery/pivot path that injects a structured
   observation before hard-aborting, then rerun at least
   `break-filter-js-from-html` and `build-cython-ext`.
3. If the 20260503T034311Z job still exists outside this worktree, import its
   `agent/sessions/*.jsonl` into a tracked execution artifact and rerun the
   event 11700-11797 analysis.

## Verification

Commands run:

```sh
find . -path '*benchmark-results*' -o -path '*harbor-jobs*' -o -path '*sessions*'
rg -n "mkdir -p /tests|break-filter-js-from-html|build-cython-ext|identical tool calls|ErrToolCallLoop|38305|400845" .ddx/attachments .fizeau .ddx/executions docs scripts -S
curl -sS --max-time 3 http://vidar:1235/v1/models
timeout 1200s env FIZEAU_BENCH_TASK=break-filter-js-from-html FIZEAU_PROVIDER_NAME=vidar-omlx FIZEAU_PROVIDER=omlx FIZEAU_MODEL=Qwen3.6-27B-MLX-8bit FIZEAU_BASE_URL=http://vidar:1235/v1 FIZEAU_API_KEY_ENV=OMLX_API_KEY FIZEAU_REASONING=medium FIZEAU_BENCH_PRESET=benchmark ./scripts/benchmark/smoke_run.sh
ps -ef | rg 'smoke-break-filter-js-from-html-20260504T031109Z|3055036|3055898|3055924|break-filter-js-from-html__EshxKtD' | rg -v 'rg' || true
docker ps --format '{{.ID}} {{.Names}} {{.Status}}' | rg 'break-filter-js-from-html|EshxKtD' || true
```
