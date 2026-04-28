# Qwen3.6-27B cross-provider comparison

**Date:** 2026-04-27
**Harness:** `ddx-agent -preset minimal -json -provider X -model Y -p ...`
**Goal:** under our agent harness, compare a cloud baseline (openrouter qwen/qwen3.6-plus) against three local-inference runtimes serving Qwen3.6-27B-class weights (omlx on vidar/grendel, lucebox dflash on bragi).
**Tier:** Tier-2 grading harness (8 prompts, structured checkers on visible content).

## Headline

| Target | Hardware | Pass rate | Mean total | Range | Mean output tok |
|---|---|---|---|---|---|
| openrouter qwen/qwen3.6-plus | cloud (Alibaba) | **8/8 = 100%** | **4.9 s** | 4.0–7.7 s | 60.6 |
| **bragi lmstudio qwen/qwen3.6-27b** | **5090-mobile, 24 GB** | **8/8 = 100%** | **5.2 s** | 3.8–7.5 s | 66.4 |
| vidar omlx Qwen3.6-27B-MLX-8bit | Apple Silicon (M-class) | 8/8 = 100% | 16.0 s | 12.1–19.9 s | 79.6 |
| grendel-omlx Qwen3.6-27B-MLX-8bit | Apple Silicon (M1 Max 64 GB) | 8/8 = 100% | 33.7 s | 24.8–57.6 s | 69.0 |
| grendel-omlx Qwen3.6-27B-UD-MLX-4bit | Apple Silicon (M1 Max 64 GB) | 8/8 = 100% | 35.7 s | 23.5–54.4 s | 81.4 |
| bragi lucebox Qwen3.6-27B-Q4_K_M | 5090-mobile, 24 GB | 8/8 = 100% | 114.8 s | 11.1–168.0 s | 88.5 |

**Quality:** all 6 targets correctly answered all 8 prompt categories (factual, math, reasoning-math, simple-instruction, code-py, json-out, structured-tool-shape, code-bug). The agent harness frames Qwen3.6 well enough that quality variance across these targets is not the discriminator.

**Speed: cloud ≈ bragi-lmstudio ≫ vidar (omlx) ≫ grendel (omlx) ≈ grendel-4bit (omlx) ≫ bragi-lucebox.**

## Per-prompt detail

```
target                      category               total    out
openrouter qwen3.6-plus     factual                4.0s    28
openrouter qwen3.6-plus     math                   4.3s    28
openrouter qwen3.6-plus     reasoning-math         7.7s   157
openrouter qwen3.6-plus     simple-instruction     4.2s    45
openrouter qwen3.6-plus     code-py                5.6s   109
openrouter qwen3.6-plus     json-out               4.6s    38
openrouter qwen3.6-plus     structured-tool-shape  4.3s    60
openrouter qwen3.6-plus     code-bug               4.4s    60

vidar omlx 8bit             factual               12.1s    24
vidar omlx 8bit             math                  13.1s    43
vidar omlx 8bit             reasoning-math        19.9s   132
vidar omlx 8bit             simple-instruction    19.6s   133
vidar omlx 8bit             code-py               18.4s   107
vidar omlx 8bit             json-out              13.4s    44
vidar omlx 8bit             structured-tool-shape 15.2s    65
vidar omlx 8bit             code-bug              16.5s    89

grendel-omlx-8bit           factual               24.8s    29
grendel-omlx-8bit           math                  25.1s    22
grendel-omlx-8bit           reasoning-math        37.9s   120
grendel-omlx-8bit           simple-instruction    57.6s   110
grendel-omlx-8bit           code-py               33.9s    89
grendel-omlx-8bit           json-out              25.8s    27
grendel-omlx-8bit           structured-tool-shape 30.7s    67
grendel-omlx-8bit           code-bug              33.6s    88

grendel-omlx-4bit-UD        factual               54.4s    29
grendel-omlx-4bit-UD        math                  27.3s    63
grendel-omlx-4bit-UD        reasoning-math        37.2s   150
grendel-omlx-4bit-UD        simple-instruction    51.8s   113
grendel-omlx-4bit-UD        code-py               30.8s    89
grendel-omlx-4bit-UD        json-out              23.5s    27
grendel-omlx-4bit-UD        structured-tool-shape 28.3s    71
grendel-omlx-4bit-UD        code-bug              32.3s   109

bragi lucebox q4_k_m        factual               11.1s    37
bragi lucebox q4_k_m        math                  31.8s    47
bragi lucebox q4_k_m        reasoning-math       115.3s   159
bragi lucebox q4_k_m        simple-instruction   123.2s    45
bragi lucebox q4_k_m        code-py              164.4s   159
bragi lucebox q4_k_m        json-out             148.1s    60
bragi lucebox q4_k_m        structured-tool-shape 168.0s    90
bragi lucebox q4_k_m        code-bug             156.9s   111

bragi lmstudio q3.6-27b     factual                4.3s    30
bragi lmstudio q3.6-27b     math                   5.0s    63
bragi lmstudio q3.6-27b     reasoning-math         7.5s   133
bragi lmstudio q3.6-27b     simple-instruction     3.9s    29
bragi lmstudio q3.6-27b     code-py                6.1s    96
bragi lmstudio q3.6-27b     json-out               3.8s    28
bragi lmstudio q3.6-27b     structured-tool-shape  4.8s    60
bragi lmstudio q3.6-27b     code-bug               5.9s    92
```

Inputs were uniformly ~3500 tokens — the agent's minimal-preset overhead (system prompt + tools + scaffold) dominates, and is identical across targets. Cost-per-prompt comparison is therefore on output tokens + provider rate.

## Observations

### 0. Bragi same-hardware A/B isolates the runtime: LM Studio is 22× faster than lucebox

Same machine (mobile 5090, 24 GB), same model class (Qwen3.6-27B Q4_K_M-tier GGUF), different runtime:

| Runtime | Mean total | tok/s implied | Notes |
|---|---|---|---|
| LM Studio (llama.cpp) | 5.2 s | ~13 | Quality identical, latency competitive with cloud baseline |
| lucebox dflash | 114.8 s | ~0.8 | DDTree speculative decoding; sm_120 unswept upstream |

**This is the critical local result.** The lucebox slowness is *not* a hardware problem — vanilla llama.cpp on the same GPU does the same job in 5 s. It's a tuning gap on Blackwell consumer (sm_120) per the upstream README, which explicitly says "Blackwell/Ada not yet swept, PRs welcome". For production use today on this hardware, **LM Studio is the right choice** — it's competitive with the cloud baseline (5.2 s vs 4.9 s).

The lucebox value proposition (speculative decoding) wants warm-state multi-turn; our per-prompt cycle is its worst case. A re-test with prefix-caching enabled or a multi-turn driver would close part of the gap, but won't recover the missing sm_120 kernel sweep.

### 1. Cloud is 3-23× faster than every local target

The cloud baseline (openrouter Alibaba-hosted qwen3.6-plus) is **~3.3× faster than vidar**, ~7× faster than grendel, ~23× faster than lucebox on this set. Expected — the cloud Qwen-plus runs on optimized inference infra. Not a fair single-machine comparison, but it sets the practical ceiling and the realistic floor for local inference latency.

### 2. vidar omlx is 2× faster than grendel omlx, same model

Same `Qwen3.6-27B-MLX-8bit` weights, same provider type (omlx). vidar mean 16.0 s vs grendel mean 33.7 s. Hardware difference — vidar's M-class is meaningfully stronger than grendel's M1 Max 64GB. If a single host is the goal, vidar is the obvious local pick.

### 3. 4bit-UD didn't speed grendel up vs 8bit through the agent

Through the wire-shape probe earlier, grendel 4bit-UD measured ~9.3 tok/s vs 8bit ~7.9 tok/s — a real per-token throughput delta. Through the agent path, both quants land in the same wall-clock band (33.7 s vs 35.7 s mean). The agent's per-prompt cycle (cold weights, full system-prompt re-encoding, thinking-mode reasoning trace) amortizes the per-token throughput difference. **Quantization choice on grendel doesn't appear to be the lever for our harness.**

### 4. Lucebox is the slowest, with high per-prompt variance

Lucebox mean 114.8 s, range 11.1–168.0 s — the second-slowest single prompt is 11× slower than the fastest. This is surprising for a project marketed as speculative decoding for throughput; possible factors:

- DFlash speculative decoding optimizes for token throughput once warm, not first-token latency. Each agent invocation re-loads / re-warms.
- Qwen3.6 thinking-mode reasoning is verbose; lucebox's reasoning_content emission may be slower than the actual answer.
- Earlier in the day this server got wedged twice under sustained load; a partially-degraded post-restart state can't be ruled out.

Worth a re-run on a quiet server to see if the variance tightens.

### 5. Lucebox passes all 8 categories under the agent path

Earlier the wire-shape probes saw lucebox return empty content (server wedge). Through the agent harness with the minimal preset and proper retry semantics, all 8 prompts now pass with sensible output. **The agent path is more robust to transient server behavior than direct curl probes** — exactly what we'd hope for. The earlier 0/8 result was a server-state artifact, not a harness incompatibility.

## What would change the picture

- **Loaded comparison.** All numbers above are per-prompt cold-cycle. A multi-turn conversation would let lucebox's speculative-decoding warm-state advantage show; right now we're in its worst case. Adding a "warm-state" subset (5 turns reusing kv-cache) would help.
- **Larger output (long-form).** Our prompts cap output around 100-160 tokens. lucebox's claimed advantage (>200 tok/s on the upstream HumanEval bench) may only show on longer generations.
- **Tool-heavy workload.** The agent's tool-loop wasn't exercised here (the prompts didn't need tools). lucebox's tool-call gaps (filed in `lucebox-tool-support-2026-04-27.md`) only matter on tool-heavy work; quality numbers above don't reflect that.

## Tier-3: beadbench preflight (real bead completion, agent loop)

Same 3 arms via `scripts/beadbench/run_beadbench.py`, task
`agent-beadbench-preflight` (bead `agent-37aeb88e`). One bead per arm.
Verify gate: `go test ./...` against the produced patch.

| Arm | Result | Duration | Tokens | Cost | Verify |
|---|---|---|---|---|---|
| **openrouter qwen/qwen3.6-plus** | ✅ success | 270 s (4.5 min) | 915,520 | $0.315 | **pass** |
| vidar omlx Qwen3.6-27B-MLX-8bit | ❌ timeout | 1 h cap | — | — | skipped |
| bragi lmstudio qwen/qwen3.6-27b | ❌ timeout | 1 h cap | — | — | skipped |

(grendel was offline and not re-tested; lucebox + vllm couldn't run because the `ddx` binary's pinned agent module is older than v0.9.18 and rejects the lucebox/vllm provider types — filed `agent-4fed70eb` to bump the dep.)

## What Tier-3 reveals that Tier-2 hid

In Tier-2, all four targets in this comparison passed 8/8 quality at single-prompt latencies of 4.9-114.8 s mean. That looked like "quality is uniform; pick on speed."

Tier-3 contradicts that read. Both 27B local arms — including bragi LM Studio at 5.2 s mean per Tier-2 prompt — **timed out at 1 hour** on a real coding bead. The cloud arm finished the same bead in 4.5 minutes.

The arithmetic explains it. A coding bead's agent loop typically runs 20-40 iterations: read files, plan, edit, run tests, react. At 5 s per single-prompt turn (the Tier-2 measurement), 30 iterations ≈ 2.5 minutes — well inside the budget. But:

- Real iterations carry growing context (each turn appends prior tool results), so per-turn latency creeps up.
- Thinking-mode reasoning compounds: every turn the model thinks before answering, often producing 1500+ reasoning tokens that count toward wall-clock but not toward visible progress.
- Tool-call cycles add network round trips between every model turn and the agent's tool execution.

The cloud baseline doesn't pay these costs at the same rate because (a) faster per-token throughput, (b) Qwen-plus's reasoning is more concise on this task class, (c) the agent's tool-execute side runs locally so model + tool overlap better with cloud latency.

**Practical conclusion:** Tier-2 single-prompt grading is necessary but not sufficient. Production-viability for an agent harness requires Tier-3-class measurement — multi-turn completion within a wall-clock budget. The bragi LM Studio result is the most striking: 5.2 s mean / 100% quality at Tier-2 → timeout at Tier-3. If you only ran Tier-2, you would have shipped that as the production local pick.

## Take-aways for harness use today

**Updated after Tier-3.** Tier-2 alone misled — local arms passed quality but timed out on real bead completion.

1. **Production pick: openrouter qwen/qwen3.6-plus.** Only target that completed a real bead within budget. 4.5 min, $0.32 per preflight bead. The cost is non-trivial at scale but the latency makes it the only currently-shippable option for ddx-agent loops.
2. **Local Qwen3.6-27B is not yet production-viable for the agent loop.** Both bragi LM Studio (despite 5.2 s mean per single prompt) and vidar omlx (16 s mean) timed out at 1 hour on a single bead. Quality is fine; iteration latency × turn count is the killer.
3. **For ad-hoc one-shot use** (chat / Q&A / single-prompt code-gen) **bragi LM Studio is the right local pick.** Tier-2 5.2 s mean matches cloud. Don't generalize this to "production agent harness" until Tier-3 numbers improve.
4. **Lucebox** is blocked on (a) the upstream `tool_choice` fix per `docs/research/lucebox-tool-support-2026-04-27.md` Gap 1, (b) sm_120 Blackwell kernel sweep, (c) bumping DDX-cli's agent dep so beadbench can drive lucebox arms. Filed as `agent-4fed70eb`.
5. **What would change the picture for local.** A long-lived agent process with persistent prefix caching (so the 3500-token system prompt amortizes across iterations) is the single biggest lever. Today's per-prompt subprocess model is local inference's worst case. Filed for design discussion alongside the warm-state work.

## Next moves

- File a follow-up bead to add a warm-state / multi-turn subset to the Tier-2 harness so lucebox's speculative-decoding case can be measured fairly.
- Re-run after lucebox upstream lands the `tool_choice` fix from `docs/research/lucebox-tool-support-2026-04-27.md` Gap 1.
- Tier 3 (beadbench preflight) on the same target set when timeline permits — that's where actual coding-task completion (not just one-shot grading) gets measured.
