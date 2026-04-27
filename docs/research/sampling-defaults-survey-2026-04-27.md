# Sampling Defaults & Recommended Profiles Survey

Date: 2026-04-27
Author: research agent
Goal: build a `(server, model-family) -> sampling profile` catalog so the
agent harness stops sending greedy `T=0` to local servers, which causes
deterministic loops on reasoning-mode models (notably Qwen3.x).

---

## Section 1 — Server Defaults

Each row is what the server applies when the client OMITS the field on a
`/v1/chat/completions` call. "Top-level non-OpenAI fields" lists which of
`top_k`, `min_p`, `repetition_penalty` are accepted as plain top-level
JSON keys (vs. needing a vendor-specific wrapper like `extra_body` or
`options`).

### oMLX (jundot/omlx, Apple Silicon MLX server)

oMLX wraps `mlx-lm`'s server. Its README does not enumerate defaults, but
it advertises drop-in OpenAI/Anthropic compat and supports per-model
sampling overrides via the admin panel. Defaults inherited from
[`mlx-lm` SERVER.md](https://github.com/ml-explore/mlx-lm/blob/main/mlx_lm/SERVER.md):

| param              | default | greedy at T=0?                |
|--------------------|---------|-------------------------------|
| temperature        | 0.0     | yes — true argmax             |
| top_p              | 1.0     |                               |
| top_k              | 0 (off) |                               |
| min_p              | 0.0     |                               |
| repetition_penalty | 0.0 (off) |                             |
| frequency_penalty  | 0.0     |                               |
| presence_penalty   | 0.0     |                               |

Top-level non-OpenAI fields accepted: **top_k YES, min_p YES, repetition_penalty YES** (per mlx-lm SERVER.md).
Does NOT auto-apply `generation_config.json` from the model — caller
must pass values explicitly. **This is the key footgun: omitting fields
gives you greedy decoding, not the model card defaults.**

Source: [mlx-lm SERVER.md](https://github.com/ml-explore/mlx-lm/blob/main/mlx_lm/SERVER.md), [omlx repo](https://github.com/jundot/omlx).

### LM Studio

| param              | default | notes                           |
|--------------------|---------|---------------------------------|
| temperature        | 1.0     |                                 |
| top_p              | 1.0     |                                 |
| top_k              | unknown (model-preset driven)   |
| min_p              | unknown (model-preset driven)   |
| repetition_penalty | unknown                         |
| frequency_penalty  | 0.0     |                                 |
| presence_penalty   | 0.0     |                                 |

Top-level non-OpenAI fields accepted: **top_k YES, min_p YES, repeat_penalty YES** (LM Studio extends OpenAI schema; documented under `LLMPredictionConfigInput`). LM Studio also applies its bundled "Preset" per model, which overrides default chat template + samplers — but only for in-app chat. Per [issue #1389](https://github.com/lmstudio-ai/lmstudio-bug-tracker/issues/1389), the API path silently ignores the `preset` field for sampling, so the server uses the active loaded-model preset only if no explicit param is sent. T=0 is greedy.

Sources: [LM Studio config docs](https://lmstudio.ai/docs/typescript/llm-prediction/parameters), [bug 1389](https://github.com/lmstudio-ai/lmstudio-bug-tracker/issues/1389).

### vLLM

| param              | default |
|--------------------|---------|
| temperature        | 1.0     |
| top_p              | 1.0     |
| top_k              | -1 (off)|
| min_p              | 0.0     |
| repetition_penalty | 1.0     |
| frequency_penalty  | 0.0     |
| presence_penalty   | 0.0     |

Top-level non-OpenAI fields accepted: **top_k YES, min_p YES, repetition_penalty YES** (vLLM extends OpenAI body; also via `extra_body`). T=0 is documented as "greedy sampling." vLLM does NOT auto-apply HF `generation_config.json` for the OpenAI server unless launched with `--generation-config auto` (added in v0.6.x); without that flag it uses the table above.

Source: [vLLM SamplingParams](https://docs.vllm.ai/en/v0.6.4/dev/sampling_params.html), [issue #11861](https://github.com/vllm-project/vllm/issues/11861).

### llama.cpp / llama-server

| param              | default          |
|--------------------|------------------|
| temperature        | 0.80             |
| top_p              | 0.95             |
| top_k              | 40               |
| min_p              | 0.05             |
| repeat_penalty     | 1.00 (disabled)  |
| frequency_penalty  | 0.0              |
| presence_penalty   | 0.0              |
| typical_p          | 1.0              |
| sampler chain      | dry, top_k, typ_p, top_p, min_p, xtc, temperature |

Top-level non-OpenAI fields accepted on `/v1/chat/completions`: **top_k YES, min_p YES, repeat_penalty YES** (llama.cpp adds them as extensions). Does NOT read HF `generation_config.json` (GGUF metadata only; see [llama.cpp discussion #17088](https://github.com/ggml-org/llama.cpp/discussions/17088) for an open proposal). T=0 → greedy.

Source: [llama.cpp server README](https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md).

### Ollama (OpenAI-compatible endpoint)

| param              | default                   |
|--------------------|---------------------------|
| temperature        | 0.8 (Ollama global default; may be overridden by Modelfile) |
| top_p              | 0.9                       |
| top_k              | 40 (native API only)      |
| min_p              | 0.0 (native API only)     |
| repeat_penalty     | 1.1 (native API only)     |
| frequency_penalty  | 0.0                       |
| presence_penalty   | 0.0                       |

Top-level non-OpenAI fields on the OpenAI endpoint: **top_k NO, min_p NO, repetition_penalty NO** — must be set in the Modelfile or sent via the native `/api/chat` endpoint's `options` object. Modelfile `PARAMETER` lines DO override defaults per-model, and many community Modelfiles (e.g. Qwen3, gpt-oss) bake in the model-card recommendations. T=0 → greedy. See [ollama#11725](https://github.com/ollama/ollama/issues/11725) for gpt-oss top_p missing from Modelfile.

Source: [Ollama OpenAI compat docs](https://docs.ollama.com/api/openai-compatibility).

---

## Section 2 — Model Family Recommended Sampling

When the model card gives separate "thinking" vs "non-thinking" recipes,
both are listed. "Tool-use" notes call out cases where the card prescribes
a different recipe for agentic / function-calling work.

| family                       | temp | top_p | top_k | min_p | rep_pen | source / notes |
|------------------------------|------|-------|-------|-------|---------|----------------|
| **Qwen2.5** (incl. 27B/Coder)| 0.7  | 0.8   | 20    | 0     | 1.05    | [Qwen2.5-Coder generation_config](https://huggingface.co/Qwen/Qwen2.5-Coder-7B-Instruct/blob/main/generation_config.json); chat models same minus `repetition_penalty=1.0` |
| **Qwen3 thinking**           | 0.6  | 0.95  | 20    | 0     | 1.0     | [Qwen3 model card best practices](https://huggingface.co/Qwen/Qwen3-8B). DO NOT use greedy. |
| **Qwen3 non-thinking**       | 0.7  | 0.8   | 20    | 0     | 1.0     | same |
| **Qwen3.5 thinking, general**| 1.0  | 0.95  | 20    | 0     | 1.0     | [Qwen3.5-9B card](https://huggingface.co/Qwen/Qwen3.5-9B); presence_penalty=1.5 recommended for general-task thinking mode |
| **Qwen3.5 thinking, code**   | 0.6  | 0.95  | 20    | 0     | 1.0     | same — coding prefers lower temp |
| **Qwen3.5 instruct**         | 0.7  | 0.8   | 20    | 0     | 1.0     | same; presence_penalty=1.5 |
| **Qwen3.6 thinking, general**| 1.0  | 0.95  | 20    | 0     | 1.0     | [Qwen3.6-27B card](https://huggingface.co/Qwen/Qwen3.6-27B), [Qwen3.6-35B-A3B card](https://huggingface.co/Qwen/Qwen3.6-35B-A3B) |
| **Qwen3.6 thinking, code**   | 0.6  | 0.95  | 20    | 0     | 1.0     | same |
| **Qwen3.6 instruct**         | 0.7  | 0.80  | 20    | 0     | 1.0     | same; presence_penalty=1.5 |
| **Qwen3-Coder** (3.x variants)| 0.7 | 0.8   | 20    | 0     | 1.05    | [Qwen3-Coder-30B-A3B-Instruct card](https://huggingface.co/Qwen/Qwen3-Coder-30B-A3B-Instruct); non-thinking only |
| **MiniMax M2 / M2.1 / M2.5 / M2.7** | 1.0 | 0.95 | 40 | unknown | unknown | [MiniMax-M2.7 card](https://huggingface.co/MiniMaxAI/MiniMax-M2.7); same recipe across the line |
| **GPT-OSS 20B / 120B**       | 1.0  | 1.0   | unknown | unknown | unknown | [openai/gpt-oss repo](https://github.com/openai/gpt-oss); cards explicitly say T=1.0, top_p=1.0 |
| **Llama 3.1 / 3.3 instruct** | 0.6  | 0.9   | unknown | unknown | unknown | [Llama generation.py](https://github.com/meta-llama/llama3/blob/main/llama/generation.py); 3.3-70B `generation_config.json` matches |
| **DeepSeek-R1**              | 0.6  | 0.95  | unknown | unknown | unknown | [DeepSeek-R1 card](https://huggingface.co/deepseek-ai/DeepSeek-R1) — must stay in 0.5-0.7 to avoid loops; greedy explicitly forbidden |
| **DeepSeek-V3**              | 0.3  | unknown | unknown | unknown | unknown | [DeepSeek API param guide](https://api-docs.deepseek.com/quick_start/parameter_settings) (coding/math preset 0.0; chat 1.3; recommended 0.3 for general) |
| **Gemma 3**                  | 1.0  | 0.95  | 64    | 0.0   | unknown | [gemma-3 inference settings](https://huggingface.co/google/gemma-3-12b-it/discussions/25) |
| **Gemma 4**                  | 1.0  | 0.95  | 64    | unknown | unknown | [Gemma 4 model card](https://ai.google.dev/gemma/docs/core/model_card_4) |

### Tool-use / agentic mode caveats

- **Qwen3 / 3.5 / 3.6**: tool-calling works in either mode but the card recommends *thinking-mode* sampling (T=0.6, top_p=0.95) for tool use; presence_penalty should drop to 0 during tool use to avoid penalising re-emission of structured tokens.
- **Qwen3-Coder**: tool-use is its primary mode; the recommended profile already assumes agentic use (T=0.7, top_p=0.8, rep_pen=1.05). No separate tool-use recipe.
- **DeepSeek-R1**: card recommends *no system prompt* and *no greedy* during tool use; same T=0.6 / top_p=0.95.
- **GPT-OSS**: OpenAI's harmony format does not change sampling recommendation for tool use; T=1.0 / top_p=1.0 throughout.
- **MiniMax M2.x**: card recommends the same T=1.0 / top_p=0.95 / top_k=40 stack including tool-use. Lower T causes loop on long agent traces (per Unsloth notes).
- **Llama 3.1/3.3**: no tool-use-specific recipe in card; 0.6/0.9 is used for both chat and tool calls.
- **Gemma 3/4**: same recipe for tool use.

---

## 5-line summary of surprising defaults

1. **oMLX (and underlying mlx-lm) defaults to T=0.0 (greedy) when the client omits temperature** — this directly causes the Qwen3.x reasoning loops we've seen in beadbench; every other server defaults to T≥0.7. This is the single biggest harness bug.
2. **Ollama's OpenAI endpoint silently drops `top_k`, `min_p`, and `repetition_penalty`** — Modelfile is the only way to set them, so our harness can think it's sending Qwen3-recommended top_k=20 and have it ignored.
3. **vLLM does NOT apply HF `generation_config.json` by default** — needs `--generation-config auto`; without it the model runs at T=1.0/top_p=1.0 regardless of what the Qwen card says.
4. **llama-server's defaults already include sane min_p=0.05 + top_k=40 + T=0.8**, so omitting fields there is much safer than on oMLX/vLLM/LM Studio — different servers fail differently.
5. **GPT-OSS recommends T=1.0/top_p=1.0** (no nucleus filtering at all), which is the opposite extreme of the Qwen3 thinking recipe (T=0.6/top_p=0.95/top_k=20); a single "one size fits all" harness preset will mis-serve at least one of these families.
