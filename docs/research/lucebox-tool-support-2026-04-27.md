# Lucebox tool-calling integration report

**Server tested:** `lucebox-hub` dflash server, `bragi:1236`, model `Qwen3.6-27B-Q4_K_M` (drop-in target on the qwen35-arch GGUF path)
**Date:** 2026-04-27
**Client:** ddx-agent conformance suite (Go, openai-go SDK; raw curl probes for verification)
**Network:** all probes from a single client; no concurrent traffic during measurement

## Summary

Wire-level tool calling **works correctly when invoked**. The OpenAI-compat shape on the response side is conformant: `tool_calls` array with `id`, `type=function`, `function.name`, `function.arguments` (as a JSON-encoded string), and `finish_reason="tool_calls"`. Streaming delivers the tool_call in a delta chunk plus a separate finish chunk plus a usage chunk in the standard order.

**One critical gap** that prevents the server being a drop-in OpenAI-compat target: `tool_choice` is silently ignored regardless of value (`"required"`, `"auto"`, named-function selector). When set, the server burns the entire `max_tokens` budget thinking but returns nothing visible.

## ✅ What works

### W1 — Standard tool call with all required args present (non-streaming)

Model emits a clean tool_call when the prompt supplies enough context to instantiate every required parameter.

```bash
curl -X POST http://bragi:1236/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model":"Qwen3.6-27B-Q4_K_M",
    "messages":[{"role":"user","content":"call the inspect tool with target=widget"}],
    "tools":[{"type":"function","function":{"name":"inspect","description":"Inspect a named test target.","parameters":{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}}}],
    "max_tokens":1024
  }'
```

Response (correct):

```json
{
  "choices":[{
    "finish_reason":"tool_calls",
    "message":{
      "content":null,
      "tool_calls":[{"id":"call_2f6ef9c1e2cc452f84b82ef5","type":"function","function":{"name":"inspect","arguments":"{\"target\": \"widget\"}"}}]
    }
  }],
  "usage":{"prompt_tokens":274,"completion_tokens":87,"total_tokens":361}
}
```

87 completion tokens, ~3 s wall-clock, well-formed call.

### W2 — Streaming tool call (same prompt, `stream:true`)

Same probe as W1 with `stream:true` + `stream_options:{include_usage:true}`. Result: 65 chunks total, 1 carrying the tool_call, then a separate finish chunk, then a usage chunk. Order:

1. `delta:{role:"assistant"}` (chunk 1)
2. ~60 `delta:{reasoning_content:"..."}` chunks (Qwen3.6 thinking trace)
3. **One** `delta:{tool_calls:[{index:0, id:"call_aed41f64e064492082e74e0b", type:"function", function:{name:"inspect", arguments:"{\"target\": \"widget\"}"}}]}`
4. `delta:{}, finish_reason:"tool_calls"`
5. `choices:[], usage:{prompt_tokens:274, completion_tokens:87, total_tokens:361}`

**Note for the lucebox team:** vLLM / OpenAI typically stream `function.arguments` incrementally across multiple deltas (e.g., `"{\"target\":"`, `"\"widget\"}"`). Lucebox sends the complete arguments in a single delta chunk. **Both are spec-compliant** per the OpenAI streaming format — clients must accumulate, but a single chunk is a degenerate-but-valid case. Worth noting if you ever get a bug report from a buggy client that assumes multiple deltas.

### W3 — Multi-tool selection

Three tools defined; directive prompt. Model picks one and emits a clean call.

```bash
curl -X POST http://bragi:1236/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"Qwen3.6-27B-Q4_K_M","messages":[{"role":"user","content":"conformance: pick a tool and call it"}],"tools":[{"type":"function","function":{"name":"inspect","description":"Inspect a named test target.","parameters":{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}}},{"type":"function","function":{"name":"summarize","description":"Summarize a piece of text.","parameters":{"type":"object","properties":{"text":{"type":"string"},"max_words":{"type":"integer"}},"required":["text"]}}},{"type":"function","function":{"name":"count_words","description":"Count the number of words in a string.","parameters":{"type":"object","properties":{"input":{"type":"string"}},"required":["input"]}}}],"max_tokens":1024}'
```

Result: `finish_reason:"tool_calls"`, model picked `count_words` with `{"input":"Hello world"}`. 224 completion tokens. Wire shape correct.

### W4 — Tool-result roundtrip

Conversation: `user → assistant tool_call → tool result → ask for final answer`. Server correctly does **not** re-emit a tool_call on the follow-up turn; emits visible content acknowledging the result.

## 🚫 Gap 1 — `tool_choice` parameter is silently ignored (most actionable)

This is the load-bearing bug. The OpenAI Chat Completions API supports `tool_choice` to control whether the model is allowed/forced/blocked from calling a tool. Lucebox accepts the field syntactically but applies no behavior change.

### Reproduction A: `tool_choice: "required"`

```bash
curl -X POST http://bragi:1236/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model":"Qwen3.6-27B-Q4_K_M",
    "messages":[{"role":"user","content":"conformance: call the inspect tool"}],
    "tools":[{"type":"function","function":{"name":"inspect","description":"Inspect a named test target.","parameters":{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}}}],
    "tool_choice":"required",
    "max_tokens":1024
  }'
```

**Expected (per OpenAI spec):** model is forced to emit a tool_call; if required args are unspecified, the model fabricates plausible defaults rather than escaping into content.

**Observed:**

```json
{
  "choices":[{
    "finish_reason":"stop",
    "message":{"content":null, "tool_calls":null}
  }],
  "usage":{"completion_tokens":1024}
}
```

Model burned the entire 1024-token budget (mostly in `reasoning_content`) and returned nothing. No tool_call. No visible content. No error.

### Reproduction B: `tool_choice: {type:"function", function:{name:"inspect"}}` (named-function selector)

Same prompt and tool definition; `tool_choice` set to the OpenAI named-function form to force a specific tool.

**Expected:** model emits a tool_call for `inspect` specifically (model may fabricate args if required ones are unspecified, or 400 if that's not supported).

**Observed:** identical to Reproduction A. `finish_reason:"stop"`, `tool_calls:null`, 1024 completion_tokens consumed, no error.

### What we'd like

**Either** honor it — wire `tool_choice` through to the chat template / sampler. Standard values:

- `"none"` — block tool emission
- `"auto"` (default) — current behavior
- `"required"` — force any tool
- `{"type":"function","function":{"name":"X"}}` — force a specific tool

**Or** return `400` with `{"error":{"code":"unsupported_parameter","param":"tool_choice"}}` so clients can detect non-support and fall back. **Silent ignore is the worst option** — produces empty 1024-token responses that look like model failures, hard to debug from the client side.

Reference: [OpenAI Chat Completions API — `tool_choice`](https://platform.openai.com/docs/api-reference/chat/create#chat-create-tool_choice).

## 🟡 Gap 2 — Auto mode is conservative; under-specified prompts escape into content

This is **model behavior** (Qwen3.6 thinking-mode), not strictly a server bug, but worth flagging because it makes lucebox underperform on tool-call benchmarks vs. vLLM serving the same weights.

When `tool_choice` is unset (auto) and the user's prompt doesn't supply every required parameter, Qwen3.6 chooses to **ask the user** rather than fabricate args.

```bash
# Under-specified: required `target` not in prompt
curl -X POST http://bragi:1236/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"Qwen3.6-27B-Q4_K_M","messages":[{"role":"user","content":"conformance: call the inspect tool"}],"tools":[{"type":"function","function":{"name":"inspect","description":"Inspect a named test target.","parameters":{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}}}],"max_tokens":1024}'
```

Result:

- `finish_reason:"stop"`
- `content:"Please provide the target you would like to inspect."`
- `tool_calls:null`
- `reasoning_content` (excerpt): *"The user wants to call the `inspect` tool. I need to provide a `target` parameter for the `inspect` tool. The user has not specified a target. I should ask the user for the target."*

vLLM serving the same weights typically emits a tool_call with a fabricated/best-guess target. The behavior difference is likely:

- The chat template's tool-calling section, OR
- A system prompt the server injects that biases toward asking, OR
- Sampling parameters that suppress argument hallucination

**Suggestion:** if there's a configurable system prompt or tool-calling template, expose it. Or document the conservative-by-default stance so benchmarks know to set `tool_choice:"required"` when they want a forced call. Both make the behavior predictable.

This becomes moot once Gap 1 is fixed — clients can pass `tool_choice:"required"` to force a call.

## 📝 Minor observation — `content` is `null` not `""`

When the response carries a tool_call, `message.content` is `null` (JSON null, not omitted, not empty string). OpenAI spec allows either. Most clients handle both, but some assume `string` and treat null as a parse error. Our Go SDK handles it; flagging because openai-python and a few JS clients have had bugs here.

## 📋 Test coverage summary

Our [`internal/provider/conformance/`](https://github.com/DocumentDrivenDX/agent/tree/master/internal/provider/conformance) suite runs 10 subtests against any OpenAI-compat endpoint. Lucebox results today:

| # | Subtest | Result | Reason |
|---|---|---|---|
| 1 | `health_check` | ✅ | `/v1/models` responds |
| 2 | `model_discovery` | ✅ | model id reported |
| 3 | `non-streaming_chat` (no tools) | ✅ | content emitted |
| 4 | `streaming_chat` (no tools) | ❌ | test brittleness — asserts literal echo cue thinking-mode models don't satisfy. **Not a lucebox issue.** Filed as `agent-bcea2d77` to fix our test side. |
| 5 | `streaming_max_tokens_honored` (cap=3) | ❌ | 3-token cap incompatible with thinking. **Not a lucebox issue.** |
| 6 | `thinking_reasoning` | ✅ | `reasoning_content` delivered |
| 7 | `tool_call_streaming` (1 tool, under-specified prompt) | ❌ | Gap 2 (model declines). Will pass once we either fix Gap 1 or our test side passes `tool_choice:"required"`. |
| 8 | `non-streaming_tool_call` (1 tool, under-specified prompt) | ❌ | same root cause as #7 |
| 9 | `multi-tool_wire_shape` (3 tools, directive prompt) | ✅ | model picks one, wire correct |
| 10 | `tool_result_roundtrip` (synthesized history) | ✅ | no re-emission, content emitted |

**Once Gap 1 ships**, conformance subtests #7 and #8 will pass without changes on our side (we'll set `tool_choice:"required"` when forcing). #4 and #5 are our test-side fixes.

## 🛠️ Reproducer commands (paste-able)

```bash
# 1. ✅ Sanity — emits clean tool_call
curl -s -X POST http://localhost:1236/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"Qwen3.6-27B-Q4_K_M","messages":[{"role":"user","content":"call the inspect tool with target=widget"}],"tools":[{"type":"function","function":{"name":"inspect","description":"Inspect a named test target.","parameters":{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}}}],"max_tokens":1024}' | jq

# 2. 🚫 Gap 1 reproducer — tool_choice ignored
curl -s -X POST http://localhost:1236/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"Qwen3.6-27B-Q4_K_M","messages":[{"role":"user","content":"conformance: call the inspect tool"}],"tools":[{"type":"function","function":{"name":"inspect","description":"Inspect a named test target.","parameters":{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}}}],"tool_choice":"required","max_tokens":1024}' | jq

# 3. 🚫 Gap 1 named-function form
curl -s -X POST http://localhost:1236/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"Qwen3.6-27B-Q4_K_M","messages":[{"role":"user","content":"conformance: call the inspect tool"}],"tools":[{"type":"function","function":{"name":"inspect","description":"Inspect a named test target.","parameters":{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}}}],"tool_choice":{"type":"function","function":{"name":"inspect"}},"max_tokens":1024}' | jq
```

## Priority ask

**Gap 1** — single-issue fix that unblocks our integration and any other client that uses `tool_choice` (most agent harnesses do). Either honor or 400 — both are progress over silent ignore. Happy to test pre-release builds.
