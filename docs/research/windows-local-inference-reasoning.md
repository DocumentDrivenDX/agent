# Windows Local Inference Reasoning Controls

Date: 2026-04-23

## Question

Agent needs a Windows-local inference path that satisfies both sides of the
execute-bead contract:

- tool-compatible chat for read/write/edit/bash style agent tools
- enforceable per-request reasoning control so local Qwen/GPT-OSS models do not
  spend the whole output budget in reasoning loops

The Mac recommendation is currently OMLX for Qwen models: Vidar OMLX probes show
Qwen `enable_thinking` / `thinking_budget` controls changing behavior. LM Studio
remains useful as a broad local provider, but Bragi's Qwen GGUF evidence showed
OpenAI-compatible `/v1/chat/completions` accepting multiple reasoning control
shapes without honoring them.

## Optimization Target

This plan ranks Windows candidates by reasoning-control fidelity first, then
operational simplicity among candidates with comparable evidence, and
tool-compatible agent execution third. That is the right order for the current
benchmark question: we are trying to identify which local engines can actually
bound or disable reasoning per request, not merely which engines can already run
tools. If the question changes to "what should ship first for Windows
execute-bead regardless of reasoning fidelity," LM Studio OpenAI-compatible
would rank higher because it is already a working tool surface.

## Current Evidence

| Engine | Windows stance | Hardware/runtime prereq | Model formats | Tool-compatible chat | Reasoning off | Named levels | Token budget | Budget surface | Separated reasoning | Tools + reasoning verified together | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| LM Studio OpenAI-compatible | Windows app available; already used on Bragi | Windows desktop app; local host runtime | GGUF and other LM Studio-served local formats | Yes, via `/v1/chat/completions` | Per-model/template; Bragi Qwen GGUF is behaviorally no-op | GPT-OSS via `/v1/responses` `reasoning.effort` only | Not proven | Per-request where honored by the model template | Yes for some models | No | Treat this as a per-model classification, not an engine-wide one. Bragi `qwen/qwen3.6-35b-a3b` Q4_K_M accepted multiple control shapes and honored none. |
| LM Studio native REST | Windows app available | Windows desktop app; local host runtime | LM Studio-served local formats | Native `/api/v1/chat` docs say custom tools are not supported on that endpoint | Documented `reasoning: off` | `low`, `medium`, `high`, `on` | No numeric budget documented | Per-request | `stats.reasoning_output_tokens` | No | Good probe surface; not yet an execute-bead surface because tools are missing. |
| Ollama | Windows-supported local server | Windows local server; exact GPU/CPU envelope depends on model size | Primarily GGUF-family local models | Has chat API; agent-specific tool probe still required | API `think: false` for most thinking models | GPT-OSS uses `low`, `medium`, `high` and cannot fully disable thinking | No numeric budget documented | Per-request | `message.thinking` | No | Provisional Windows-native candidate. Operationally simpler than llama.cpp, but not ahead of it on reasoning evidence alone. |
| vLLM under WSL/Linux | Practical Windows route is WSL2, not native Windows | WSL2 plus NVIDIA GPU with CUDA passthrough and enough VRAM for the selected model/quant; if this is unavailable, skip this tier | Safetensors/transformer-style model repos, not GGUF | OpenAI-compatible server with tool-call examples | Qwen3 parser supports `enable_thinking=False` via chat template kwargs | Request-level template kwargs can override defaults | `thinking_token_budget` sampling parameter | Per-request | `message.reasoning` | Not yet in our environment | Best documented match for tool use plus numeric reasoning budgets, but only for Windows hosts that can actually run the stack. |
| llama.cpp / llama-server | Windows binaries exist | Windows-native binaries; exact GPU acceleration and memory limits depend on build/backend | GGUF | OpenAI-style function calling documented | Qwen docs and community issues discuss `enable_thinking=false`; behavior must be probed | No stable named-level contract found | Active reasoning-budget work, but per-request API support is unclear | Unclear / possibly startup-flag or build-specific | Reasoning chunks appear in server output | No | Promising Windows-native fallback, but needs live probe against the exact build/model. |

## Source Notes

- LM Studio REST overview documents `/api/v1/chat` and its endpoint comparison:
  native chat supports a request `context_length`, but custom tools are listed on
  OpenAI-compatible `/v1/responses`, `/v1/chat/completions`, and
  Anthropic-compatible `/v1/messages`, not native chat:
  <https://lmstudio.ai/docs/developer/rest-api>
- LM Studio native chat documents `reasoning` as
  `off|low|medium|high|on` and response `stats.reasoning_output_tokens`:
  <https://lmstudio.ai/docs/developer/rest/chat>
- LM Studio Responses documents `reasoning: { "effort": "low" }` for
  `openai/gpt-oss-20b`:
  <https://lmstudio.ai/docs/developer/openai-compat/responses>
- Ollama documents the `think` API field, Qwen 3 support, GPT-OSS
  `low|medium|high`, and separated `message.thinking`:
  <https://docs.ollama.com/capabilities/thinking>
- vLLM documents Qwen3 reasoning parser behavior, request-level
  `chat_template_kwargs`, tool-call examples with reasoning, and
  `thinking_token_budget`:
  <https://docs.vllm.ai/en/latest/features/reasoning_outputs/>
  <https://docs.vllm.ai/en/stable/api/vllm/reasoning/qwen3_reasoning_parser/>
- llama.cpp documents OpenAI-style function calling:
  <https://github.com/ggml-org/llama.cpp/blob/master/docs/function-calling.md>
  Its Qwen local-running docs and current project discussions indicate
  `enable_thinking` / reasoning-budget behavior is active but still needs
  build-specific validation:
  <https://qwen.readthedocs.io/en/latest/run_locally/llama.cpp.html>
  <https://github.com/ggml-org/llama.cpp/discussions/21445>

These links were checked on 2026-04-23. Several are unversioned vendor docs;
re-validate request shapes before treating this note as executable truth.

## Recommendation

Use a tiered Windows strategy:

1. Default Windows research target: vLLM in WSL2 for evidence-grade local
   execute-bead, because it is the best documented intersection of
   OpenAI-compatible tool calling, separated reasoning output, request-level
   Qwen thinking control, and numeric `thinking_token_budget`.
   Preconditions: WSL2, NVIDIA CUDA passthrough, and enough VRAM for the
   chosen model/quant. For the initial Windows path, restrict this tier to a
   Qwen3 8B-class pilot rather than jumping straight to 30B/32B-class models.
   If the host cannot run that 8B-class pilot under WSL2/CUDA in a reasonable
   setup window, skip directly to the Windows-native tier.
2. Windows-native provisional tier: Ollama and llama.cpp/llama-server. Both
   require the same live probe before they can be promoted for execute-bead.
   Start with Ollama because packaging and operator friction are lower on
   Windows; if its tool-plus-reasoning probe fails, keep llama.cpp in the same
   tier rather than treating Ollama as the default by assumption.
3. llama.cpp-specific promotion rule: treat startup-flag-only reasoning control
   as insufficient for execute-bead promotion. The bar is per-request control,
   because benchmark arms and route selection need task-level effort changes.
   A llama-server build that only supports `--reasoning-budget` at process start
   may still be useful for single-model experiments, but it does not satisfy the
   per-request capability matrix for general execute-bead routing.
4. Keep LM Studio in the support matrix, but split its capability rows:
   OpenAI-compatible chat is tool-capable but Bragi Qwen reasoning control is
   no-op in current evidence; native chat has documented reasoning control but
   is not tool-capable enough for execute-bead.

## Live Probe Plan

For each Windows candidate, run the same four checks before promoting it to an
evidence-grade arm:

1. Tool call smoke: a small OpenAI-compatible chat request with one required
   JSON function/tool call.
2. Reasoning off: Qwen request with the candidate's documented off switch;
   require non-empty content and, where the runtime exposes separated
   reasoning-token accounting, zero reasoning tokens. Engines without separated
   reasoning-token accounting (for example some llama.cpp or Ollama builds) are
   judged by visible-thinking suppression relative to the no-control baseline.
3. Reasoning budget: request a small budget; require a bounded
   reasoning-token count or an explicit documented unsupported result. For
   engines that expose reasoning-token counts, treat `reasoning_tokens <= 2 *
   budget` as the initial success rule unless the engine's own docs define a
   stricter accounting contract.
4. Agent loop smoke: one read-only execute-bead task with `reasoning=off` and
   one write task with the intended default reasoning level.

Use the same classification thresholds as
`scripts/beadbench/probe_reasoning_controls.py` so results remain comparable:

- `reasoning=off` is considered honored when `reasoning_tokens == 0`, or when
  the control removes visible thinking relative to the no-control baseline.
- A level/budget control is considered behaviorally meaningful when either
  `reasoning_chars` changes by more than 16 characters across probes or
  completion/reasoning token counts change by more than 4 tokens.
- A single probe run is classification-only. Promotion to an evidence-grade
  benchmark arm still requires the existing three-run rule from beadbench's
  evidence policy.

Suggested first commands:

```bash
# Ollama
curl http://localhost:11434/api/chat -d '{"model":"qwen3:8b","messages":[{"role":"user","content":"What is 37*42? Answer only the integer."}],"think":false,"stream":false}'

# vLLM (assumes the server was started with the Qwen3 reasoning parser)
curl http://localhost:8000/v1/chat/completions -H 'Content-Type: application/json' -d '{"model":"Qwen/Qwen3-8B","messages":[{"role":"user","content":"What is 37*42? Answer only the integer."}],"chat_template_kwargs":{"enable_thinking":false}}'

# vLLM budget probe
# Use the deployment's documented request shape for `thinking_token_budget`
# after confirming the server version and parser flags. This field is version-
# sensitive enough that the server's own docs should win over this note.

# llama-server
curl http://localhost:8080/v1/chat/completions -H 'Content-Type: application/json' -d '{"model":"qwen3.5","messages":[{"role":"user","content":"/no_think\nWhat is 37*42? Answer only the integer."}]}'
```
