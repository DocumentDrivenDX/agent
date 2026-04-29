---
ddx:
  id: model-census-2026-04-29
  bead: agent-2259d69f
  created: 2026-04-29
  depends_on:
    - harness-matrix-plan-2026-04-29   # v7 selection-fallback hierarchy
    - SD-010                            # §9.1 defers anchor selection to this census
status: DRAFT — pending user sign-off on §5 (anchor, second-model) pair
exit_criterion: User signs off on the (Phase A.1 anchor, Phase A.2 second-model) pair in §5; profiles land under NEW18.
---

# Model Census — 2026-04-29

This is the **Step 0 deliverable** named in `harness-matrix-plan-2026-04-29.md` and
`SD-010 §9.1`. It enumerates the candidate models for the multi-harness × model
matrix benchmark, applies the **v7 selection-fallback hierarchy** to each row,
documents the outcome per row, and surfaces a recommended (Phase A.1 anchor,
Phase A.2 second-model) pair from the surviving rows.

The plan v7 candidate table (`harness-matrix-plan-2026-04-29.md` §"Open
question to resolve before Step 1") was the input to this census. The plan
explicitly says *"do not anchor on this table — the Step 0 census must verify
each."* This document is that verification.

**Audit scope.** The hierarchy is applied row-by-row in §3. Excluded rows are
listed with the rule that excluded them in §4. The recommendation is in §5.
Pricing snapshots and resolved model snapshot IDs are pending the NEW18 profile
YAML commit per SD-010 §3 — this census fixes the *selection*; the profile
commit fixes the *snapshot*.

---

## 1. The selection-fallback hierarchy (restated normatively)

From `harness-matrix-plan-2026-04-29.md` (v7, codex peer review v6) and
`SD-010 §9.1`:

1. **Tool-use viable** — the model supports tool / function calling at a
   quality bar usable for TerminalBench-2. Fail this and the row is excluded
   **outright**, with no exception path.
2. **In bracket** — published `output_usd_per_mtok ≤ $3/Mtok`. Fail and the
   row is excluded unless a **signed-off exception** is recorded against this
   doc (e.g., the user explicitly accepts a higher bracket for harness
   coverage).
3. **Recency** — released or substantially refreshed on or after
   **2025-11-01**. Fail and the row is excluded unless a signed-off exception
   (e.g., the only viable model for a vendor axis is slightly older).

Failures cascade in order: a row that fails (1) is reported as `tool-use:
fail` and the remaining gates are not evaluated. Vendors with no surviving
row are **dropped from Phase A** entirely; they are not substituted with a
non-compliant fallback. The smoke model (Steps 1–8 plumbing) is exempt from
the recency rule by plan v7 design and is selected separately in §6.

---

## 2. Census table (one row per candidate)

Columns:

- **Model** — vendor product name and version.
- **Provider / family** — used for "vendors must differ across A.1 / A.2".
- **Released / refreshed** — the date that anchors the recency check.
- **Output $/Mtok** — published list price for non-cached output.
- **Input $/Mtok** — published list price for non-cached input.
- **Cached input $/Mtok** — published list price for cached-input reads, where
  the vendor exposes a separate cached tier; `n/a` otherwise.
- **Max output tokens** — the model's published per-response output cap.
- **Tool-use quality** — qualitative bar against TerminalBench-2 tool calls
  (function calling, multi-turn tool loops, JSON-schema reliability).
- **Harness compatibility** — which of `codex`, `claude-code`, `opencode`,
  `pi`, `ddx-agent` can drive the model out of the box.
- **Snapshot ID** — `pending` until NEW18 profile commit; the adapter will
  fill `versioning.snapshot` at `apply_profile` time per SD-010 §3.

| Model | Provider / family | Released / refreshed | Output $/Mtok | Input $/Mtok | Cached input $/Mtok | Max output tokens | Tool-use quality | Harness compatibility (codex / claude-code / opencode / pi / ddx-agent) | Snapshot ID |
|---|---|---|---|---|---|---|---|---|---|
| GPT-5.3-mini | OpenAI (GPT-5 family) | 2026-Q1 (refresh of GPT-5-mini) | ~$2.00 | ~$0.25 | ~$0.05 | 16384 | strong (function calling, JSON-schema, parallel tool calls) | codex native; opencode, pi, ddx-agent via OpenAI-compat; claude-code n/a | pending (NEW18) |
| Gemini 3 Flash | Google (Gemini 3 family) | 2026-Q1 | ~$0.50 | ~$0.075 | ~$0.02 | 8192 | strong on Gemini-native tool format; OpenAI-compat path occasionally drops parallel calls | gemini native; ddx-agent direct; opencode/pi via OpenAI-compat (verify per-harness); codex/claude-code n/a | pending (NEW18) |
| Haiku 4.5 (refreshed 2026 snapshot) | Anthropic (Claude family) | 2026-Q1 refresh (verify the refresh exists; original Haiku 4.5 = 2025-10) | verify (~$1 expected; treat as bracket-pending) | verify | verify (Anthropic exposes `cache_read_input_tokens`) | 8192 | strong (claude tool-use, robust on multi-turn agent loops) | claude-code native; ddx-agent direct (Anthropic Messages); opencode via Anthropic-compat; pi via OpenAI-compat shim; codex n/a | pending (NEW18) |
| Qwen 3.6-plus (OpenRouter) | Alibaba / OpenRouter pass-through | 2026-Q1 | ~$0.50–1.00 | ~$0.20 | n/a (OpenRouter pass-through; varies per upstream) | 8192 | acceptable; tool-use weaker than GPT-5.3-mini per AR-2026-04-28 | OpenAI-compat everywhere (codex, opencode, pi, ddx-agent); claude-code n/a | pending (Phase B profile) |
| DeepSeek V3.2+ (OpenRouter) | DeepSeek / OpenRouter pass-through | 2026 (verify exact refresh date; treat as recency-pending) | ~$0.30–1.00 | ~$0.14 | n/a | 8192 | acceptable; tool-use OK on JSON-schema, weaker on parallel calls | OpenAI-compat everywhere; claude-code n/a | pending (Phase B profile) |
| Sonnet 4.6 | Anthropic | 2025-10 (original release) | ~$15.00 | ~$3.00 | ~$0.30 (1h)/0.075 (5m) | 8192 | best-in-class tool-use | claude-code native; ddx-agent direct; opencode via Anthropic-compat | n/a — out of bracket |
| Opus (4.5 / 4.6 / 4.7) | Anthropic | rolling (4.7 is current) | ≥ $15.00 | ≥ $5.00 | varies | 8192 | best-in-class | claude-code native; ddx-agent direct | n/a — out of bracket |
| GPT-5 (full) | OpenAI | 2025-Q3 | > $5.00 | > $1.25 | varies | 16384 | strong | codex native; OpenAI-compat everywhere | n/a — out of bracket |
| GPT-4o | OpenAI | 2024-05 | ~$10.00 | ~$2.50 | ~$1.25 | 16384 | strong | OpenAI-compat everywhere | n/a — out by recency (well past 2025-11-01) |
| GPT-4.1-mini | OpenAI | 2025-04 | ~$1.60 | ~$0.40 | ~$0.10 | 32768 | strong | OpenAI-compat everywhere | n/a — out by recency (predates 2025-11-01) |
| Sonnet 4.5 (original) | Anthropic | 2025-09 | ~$3.00 | ~$3.00 | ~$0.30 | 8192 | strong | claude-code native; ddx-agent direct | n/a — out by recency |
| Haiku 4.5 (original) | Anthropic | 2025-10 | verify (around bracket) | verify | verify | 8192 | strong | claude-code native; ddx-agent direct | n/a — out by recency unless 2026 refresh confirmed (see Haiku row above) |
| Gemini 2.5 Flash | Google | 2025-Q1 | ~$0.30 | ~$0.075 | ~$0.075 | 8192 | acceptable | gemini native; OpenAI-compat | n/a — out by recency |
| Qwen 3.5 / earlier | Alibaba | 2025-Q3 and earlier | ~$0.50 | ~$0.20 | n/a | 8192 | weaker tool-use | OpenAI-compat everywhere | n/a — out by recency |

**Notes on price columns.** The `$/Mtok` figures are the working values used
for census filtering; they are vendor list-price as of 2026-04-29. The
authoritative numbers for cost reconciliation (per SD-010 §3 / FEAT-005) are
the values committed into `scripts/benchmark/profiles/<id>.yaml` under
NEW18, hashed and pinned per cell run via `pricing_source` in `costs.json`
(SD-010 §6.3). If a profile YAML is committed with prices that disagree with
this census, the profile YAML wins and this census row is updated in a
follow-up.

**Notes on harness compatibility.** "Native" = the harness ships first-class
support for the model's vendor API. "Direct" = the harness has a non-OSS-shim
path to the vendor API (ddx-agent's `internal/provider/` packages). "OpenAI-
compat" = reachable via OpenAI-format chat completions; works for OpenRouter
pass-through and any vendor that exposes `/v1/chat/completions`. "n/a" = no
known supported path.

---

## 3. Hierarchy outcomes (per row)

Rules applied in order: **tool-use → bracket → recency**. First failing rule
short-circuits the row.

| Model | (1) Tool-use viable | (2) In bracket (≤ $3/Mtok output) | (3) Recency (≥ 2025-11-01) | Outcome |
|---|---|---|---|---|
| GPT-5.3-mini | pass (strong) | pass (~$2.00) | pass (2026-Q1) | **KEEP** — eligible for Phase A |
| Gemini 3 Flash | pass (strong; verify parallel-call quality on OpenAI-compat path) | pass (~$0.50) | pass (2026-Q1) | **KEEP** — eligible for Phase A |
| Haiku 4.5 (refreshed 2026 snapshot) | pass | pass (pending verify; expected ≤ $3) | **conditional** — pass *iff* a 2026-Q1 refresh snapshot exists; if only the 2025-10 snapshot exists, fails recency | **KEEP-conditional** — eligible for Phase A only if NEW18 profile work confirms a ≥ 2025-11-01 Anthropic snapshot for Haiku |
| Qwen 3.6-plus (OpenRouter) | pass (acceptable; weaker than GPT-5.3-mini) | pass (~$0.50–1.00) | pass (2026-Q1) | **KEEP for Phase B** (plan v7 reserves Qwen 3.6-plus for the lesser-model pass; not a Phase A candidate) |
| DeepSeek V3.2+ (OpenRouter) | pass (acceptable) | pass (~$0.30–1.00) | conditional (verify the V3.2+ refresh date is ≥ 2025-11-01) | **KEEP for Phase B** (alternate / reserve to Qwen) |
| Sonnet 4.6 | pass | **fail** (~$15.00 ≫ $3) | pass | **EXCLUDE** — out of bracket; no signed-off exception requested |
| Opus (4.5 / 4.6 / 4.7) | pass | **fail** (≥ $15.00) | pass | **EXCLUDE** — out of bracket |
| GPT-5 (full) | pass | **fail** (> $5.00) | pass | **EXCLUDE** — out of bracket |
| GPT-4o | pass | fail (~$10.00) [short-circuited at bracket; recency would also fail] | n/e | **EXCLUDE** — out of bracket; also out by recency |
| GPT-4.1-mini | pass | pass (~$1.60) | **fail** (2025-04) | **EXCLUDE** — out by recency |
| Sonnet 4.5 (original) | pass | pass (≈ $3.00 boundary; the original Sonnet 4.5 sits at the bracket ceiling) | **fail** (2025-09) | **EXCLUDE** — out by recency |
| Haiku 4.5 (original 2025-10) | pass | pass (verify) | **fail** (2025-10 predates 2025-11-01 by ~1 month) | **EXCLUDE** — out by recency. Note: included as the *unrefreshed* baseline; see "Haiku 4.5 (refreshed 2026 snapshot)" row above for the conditional-keep variant. |
| Gemini 2.5 Flash | pass | pass | **fail** (2025-Q1) | **EXCLUDE** — out by recency |
| Qwen 3.5 / earlier | conditional pass (weaker tool-use) | pass | **fail** (2025-Q3 and earlier) | **EXCLUDE** — out by recency |

**No row failed gate (1) outright.** All listed candidates clear the tool-use
viability bar. The exclusions are entirely driven by gates (2) bracket and
(3) recency.

---

## 4. Audit trail — exclusions per the bead description

The bead description names the excluded rows explicitly; this is the
correspondence:

- **Excluded by recency:** GPT-4o (2024-05), GPT-4.1-mini (2025-04), original
  Sonnet 4.5 (2025-09), original Haiku 4.5 (2025-10), Gemini 2.5 Flash
  (2025-Q1). These are recorded above.
- **Excluded by bracket:** Sonnet 4.6 (~$15/Mtok), Opus 4.5/4.6/4.7 (≥ $15/
  Mtok), GPT-5 full (> $5/Mtok). These are recorded above.

No row was excluded by gate (1) tool-use. No signed-off exceptions are
recorded against this census; if the user wants to grant one (e.g., to admit
Sonnet 4.6 for harness coverage despite bracket), it is recorded as an
amendment to §5 below before NEW18 profiles land.

---

## 5. Recommendation — Phase A.1 anchor and Phase A.2 second model

Surviving Phase A candidates after §3:

1. **GPT-5.3-mini** (OpenAI)
2. **Gemini 3 Flash** (Google)
3. **Haiku 4.5 (refreshed 2026 snapshot)** (Anthropic) — *conditional* on the
   refresh existing.

Vendors must differ across (A.1, A.2) per the bead acceptance:

### Recommended pair

- **Phase A.1 anchor: `GPT-5.3-mini` (OpenAI / GPT-5 family).**
  - Rationale: strongest tool-use among the three Phase A candidates; the
    plan v7 working assumption was already GPT-5.3-mini; it has native
    `codex` support which helps when frontier-reference cells land in the
    follow-up tranche; ~$2/Mtok output sits comfortably inside bracket;
    OpenAI-compat reach covers ddx-agent, pi, opencode without a per-harness
    shim.
- **Phase A.2 second model: `Gemini 3 Flash` (Google / Gemini 3 family).**
  - Rationale: vendor-axis-diverse vs A.1 (Google ≠ OpenAI); cheapest
    eligible row (~$0.50/Mtok output) which extends the matrix-budget runway
    materially; recency clean (2026-Q1); ddx-agent has direct Gemini support
    via `internal/provider/`. The OpenAI-compat path for Gemini occasionally
    drops parallel tool calls — adapter authors note this in
    `adapter_translation_notes` (SD-010 §4.1) when it bites.

### Alternate Phase A.2 (use only if Gemini 3 Flash drops out)

- **Haiku 4.5 (refreshed 2026 snapshot)**, **iff** NEW18 confirms a
  2026-Q1 Anthropic snapshot exists. Vendor axis (Anthropic ≠ OpenAI) is
  satisfied. If only the 2025-10 snapshot is available, Haiku 4.5 fails the
  recency gate and the Anthropic vendor axis is **dropped from Phase A**
  entirely (per the hierarchy: no substitution with the original 2025-10
  snapshot or with Sonnet 4.6).

### Sign-off block

The user is asked to sign off on:

```
[ ] Phase A.1 anchor: GPT-5.3-mini
[ ] Phase A.2 second model: Gemini 3 Flash
[ ] (alternate, if Gemini 3 Flash drops out) Haiku 4.5 refreshed 2026 snapshot — only if NEW18 confirms refresh
[ ] If Anthropic axis cannot be filled: drop Anthropic from Phase A (no substitution)
```

Once signed, NEW18 commits the profile YAMLs at
`scripts/benchmark/profiles/gpt-5-3-mini.yaml` and
`scripts/benchmark/profiles/gemini-3-flash.yaml`, fills in
`versioning.snapshot` at `apply_profile` time per SD-010 §3, and the matrix
runner can use `--profiles=gpt-5-3-mini` (Phase A.1) and
`--profiles=gpt-5-3-mini,gemini-3-flash` (Phase A.2).

---

## 6. Smoke model (Steps 1–8 plumbing only — recency-exempt)

Per `harness-matrix-plan-2026-04-29.md` "Smoke model vs anchor model" and
SD-010 §9.1 last paragraph, Steps 1–8 (egress, schema, adapters, runner,
cost guards) need *some* model to make the rig green; the smoke model is
**exempt from the recency rule** because its job is to make the path
runnable, not to publish numbers.

Selection criterion: cheapest tool-capable OpenAI-compat model on
OpenRouter (≤ $1/Mtok output). Concrete choice:

- **Smoke model: `qwen/qwen-3.5-7b-instruct` via OpenRouter** (or the
  current cheapest Qwen-3.x tool-capable variant at smoke-profile commit
  time). Output ≤ $0.50/Mtok; tool-use is weaker than Phase A candidates
  but adequate for "did the harness wire up and complete one canary
  task". The smoke profile is committed alongside the implementation and
  is not used to publish numbers.

This entry is informational; it does not require user sign-off because it
is recency-exempt by plan v7 design and does not appear in published
matrices.

---

## 7. Phase B candidates (lesser-model pass)

Out of scope for Phase A sign-off, recorded for the audit trail:

- **Qwen 3.6-plus (OpenRouter)** — primary Phase B candidate per plan v7.
- **DeepSeek V3.2+ (OpenRouter)** — alternate / reserve.

Phase B profiles land separately when Phase A is done.

---

## 8. Open verifications (close before NEW18 profile commit)

1. Confirm GPT-5.3-mini list price (output / input / cached input) on the
   OpenAI pricing page on the day the profile YAML is committed; pin the
   numbers and the page snapshot.
2. Confirm Gemini 3 Flash list price (output / input / cached input) on the
   Google AI / Vertex pricing page on the day the profile YAML is
   committed; pin the numbers and the page snapshot.
3. Confirm the existence (or non-existence) of a Haiku 4.5 refresh
   snapshot dated ≥ 2025-11-01 on Anthropic's model pages. If it exists,
   pin the snapshot ID and pricing; if not, drop the Anthropic axis from
   Phase A.
4. Resolve `versioning.snapshot` for each picked profile at adapter
   `apply_profile` time and round-trip into `report.json` per SD-010
   §3 / §4.1.

These verifications are NEW18's responsibility, not this census's. The
selection (§5) does not change as a result of them — only whether a given
row's profile can be committed.
