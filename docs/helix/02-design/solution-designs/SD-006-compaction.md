---
ddx:
  id: SD-006
  depends_on:
    - FEAT-001
    - SD-001
---
# Solution Design: SD-006 — Conversation Compaction

## Problem

The agent loop appends every message and tool result to the conversation
history. For tasks requiring many tool-call rounds, the history will exceed
the model's context window and the provider will return an error. Local models
have especially small windows (8K-32K) making this a practical blocker.

## Research Summary

### Pi's Approach
- **Trigger**: `contextTokens > contextWindow - reserveTokens` (reserve: 16K default)
- **Keep recent**: ~20K tokens of recent messages preserved verbatim
- **Summarize**: Everything before the cut point summarized by the LLM
- **Format**: Structured markdown — Goal, Progress (checkboxes), Key Decisions, Next Steps, Critical Context
- **Update mode**: When prior summary exists, merges new info rather than re-summarizing
- **File tracking**: Accumulates read/modified file lists in XML tags on the summary
- **Tool result truncation**: 2K chars max in summarization input
- **Split turn handling**: Separate "turn prefix summary" when cut falls mid-turn

### Codex's Approach
- **Trigger**: `total_usage_tokens >= auto_compact_token_limit` (per-model, e.g., 200K)
- **Two modes**: Local (LLM-based) and remote (OpenAI server-side API)
- **Prompt**: Short — "Create a handoff summary for another LLM that will resume the task"
- **Summary injection**: "Another language model started to solve this problem..."
- **User messages**: Keeps up to 20K tokens of recent user messages alongside summary
- **Timing**: Pre-turn (before user input) and mid-turn (between tool call rounds)
- **Fallback**: Trims oldest history items if compaction prompt itself exceeds context
- **Warning**: Alerts user that multiple compactions degrade accuracy

## Design: Forge Compaction

### Strategy

Forge follows pi's structured approach (richer summaries, file tracking) with
Codex's pragmatism (mid-turn compaction, configurable thresholds, graceful
fallback). The compaction is a library feature — not just CLI — so embedders
can control it.

### Configuration

```go
type CompactionConfig struct {
    // Enabled controls whether automatic compaction runs. Default: true.
    Enabled bool

    // ContextWindow is the model's context window in tokens. If zero,
    // the provider is queried or a conservative default (8192) is used.
    ContextWindow int

    // ReserveTokens is the token budget reserved for the model's response
    // and the next prompt. Compaction triggers when conversation tokens
    // exceed ContextWindow - ReserveTokens. Default: 8192.
    ReserveTokens int

    // KeepRecentTokens is how many tokens of recent messages to preserve
    // verbatim after compaction. Default: 8192.
    KeepRecentTokens int

    // MaxToolResultTokens is the max tokens per tool result included in
    // the summarization input. Longer results are truncated. Default: 500.
    MaxToolResultTokens int

    // SummarizationModel overrides the model used for summarization.
    // If empty, uses the same model as the agent loop. Useful for using
    // a faster/cheaper model for compaction (e.g., local model for
    // summarization even when the agent uses a cloud model).
    SummarizationModel string

    // SummarizationProvider overrides the provider for summarization.
    // If nil, uses the same provider as the agent loop.
    SummarizationProvider Provider
}
```

Added to `Request`:

```go
type Request struct {
    // ...existing fields...

    // Compaction configures automatic conversation compaction.
    // If nil, compaction is enabled with defaults.
    Compaction *CompactionConfig
}
```

### Trigger Logic

```
shouldCompact = estimatedTokens > (contextWindow - reserveTokens)
```

Token estimation: use the provider's reported usage from the last response
(accurate), plus chars/4 heuristic for messages added since.

Checked at two points:
1. **Pre-iteration**: Before sending the next prompt to the model
2. **Mid-iteration**: After tool results are appended (a large bash output
   can push over the limit between iterations)

### What Gets Compacted

1. Walk backwards from newest messages, accumulating token estimates
2. Stop when `keepRecentTokens` is reached — everything after this point is kept
3. Everything before the cut point is serialized and summarized
4. The cut point must be at a message boundary (never mid-tool-call)

### Serialization for Summarization

Tool calls serialized compactly:
```
[User]: Read main.go and fix the bug
[Assistant → read(path="main.go")]: package main...
[Assistant → edit(path="main.go", old="bug", new="fix")]: Replaced 1 occurrence
[Assistant]: Fixed the bug by replacing...
```

Tool results truncated to `MaxToolResultTokens`.

### Summarization Prompt

Forge uses pi's structured format (more useful for spec-driven work) with
Codex's framing (handoff to another LLM):

```
You are performing a CONTEXT CHECKPOINT COMPACTION. Create a structured
handoff summary for another LLM that will resume this task.

Use this EXACT format:

## Goal
[What the user is trying to accomplish]

## Constraints & Preferences
- [Requirements, conventions, or preferences mentioned]

## Progress
### Done
- [x] [Completed work with file paths]

### In Progress
- [ ] [Current work]

## Key Decisions
- **[Decision]**: [Brief rationale]

## Next Steps
1. [What should happen next]

## Files
### Read
- [Files that were read]

### Modified
- [Files that were created or edited]

## Critical Context
- [Data, error messages, or references needed to continue]

Keep each section concise. Preserve exact file paths, function names,
and error messages. Do not continue the conversation — only output
the summary.
```

### Update Mode

When a previous compaction summary exists, the prompt changes to:

```
The messages above are NEW conversation since the last compaction.
Update the existing summary (provided in <previous-summary> tags)
by merging new information.

RULES:
- PRESERVE all existing information from the previous summary
- ADD new progress, decisions, and context
- UPDATE Progress: move completed items from In Progress to Done
- UPDATE Next Steps based on what was accomplished
- PRESERVE exact file paths and error messages
```

### Summary Injection

The summary replaces all compacted messages as a user-role message:

```
The conversation history before this point was compacted into the
following summary:

<summary>
{structured summary}
</summary>
```

### File Tracking

Like pi, forge tracks which files were read and modified across compactions.
The file lists are appended to the summary in XML tags and carried forward
through subsequent compactions.

### Token Counting

Three approaches, in preference order:
1. **Provider-reported usage**: From the last `Response.Usage` — most accurate
2. **Chars/4 heuristic**: For messages added since last response — conservative
3. **Configured context window**: From `CompactionConfig.ContextWindow` or
   provider metadata

### Events

New event types:
```go
EventCompactionStart EventType = "compaction.start"
EventCompactionEnd   EventType = "compaction.end"
```

The `compaction.end` event data includes the summary text, tokens before/after,
and file lists.

### Graceful Degradation

If the compaction prompt itself exceeds the context window:
1. Trim oldest messages from the summarization input
2. If still too large, fall back to aggressive truncation (keep only the
   most recent messages, drop the summarization attempt)
3. Log a warning via callback

## Implementation Plan

| # | Bead | Depends |
|---|------|---------|
| 1 | Token estimation (chars/4 + provider usage tracking) | — |
| 2 | Conversation serialization for summarization | — |
| 3 | Compaction config types and trigger logic | 1 |
| 4 | Summarization prompt and summary injection | 2, 3 |
| 5 | File tracking across compactions | 4 |
| 6 | Mid-turn compaction in agent loop | 4 |
| 7 | Update mode (merge with previous summary) | 4 |
| 8 | Integration test: multi-round task with compaction | 6 |

## Risks

| Risk | Prob | Impact | Mitigation |
|------|------|--------|------------|
| Local model produces poor summaries | M | H | Allow dedicated summarization model; structured format constrains output |
| Token estimation inaccurate | M | M | Conservative estimate (chars/4 overestimates); triggers early rather than late |
| Multiple compactions degrade quality | M | M | Warn after compaction; update mode preserves prior summary content |
| Summarization adds latency | L | M | Use faster model for summarization; only triggers when needed |
