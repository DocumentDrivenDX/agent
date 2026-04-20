---
ddx:
  id: ADR-002
  depends_on:
    - helix.prd
    - helix.arch
    - CONTRACT-003
---
# ADR-002: PTY Cassette Transport for Harness Golden Masters

| Date | Status | Deciders | Related | Confidence |
|------|--------|----------|---------|------------|
| 2026-04-20 | Accepted | DDX Agent maintainers | `CONTRACT-003`, harness capability matrix | Medium |

## Context

| Aspect | Description |
|--------|-------------|
| Problem | Real subprocess harness support needs golden-master evidence that exercises the same PTY behavior users see, but the project has not chosen whether tmux, direct PTY supervision, or a separate terminal recorder owns that lifecycle. |
| Current State | Runtime subprocess execution uses `os/exec` plus harness-specific runners. Quota probes have used tmux-shaped experiments, but normal harness execution does not have one attachable PTY transport. Existing concerns require either standardizing on tmux for the whole lifecycle or owning PTY/session supervision directly. |
| Requirements | One transport must cover live execution, record mode, replay mode, cancellation, cleanup, inspection, quota/status probing, service-event capture, and deterministic cassette playback. Record mode must fail fast on missing binary/auth/subscription/quota instead of writing misleading fixtures. |

## Decision

DDX Agent will own direct PTY lifecycle in-process using Go `os/exec` plus a
small PTY abstraction. tmux is not a required dependency for harness execution,
record mode, replay mode, quota/status probing, cancellation, or inspection.

The cassette recorder and player remain part of this repository until another
consumer needs the same API. If reuse appears, split the PTY cassette player
into a separate project only after the cassette format has one versioned
contract and at least two real consumers.

**Key Points**: Direct PTY is canonical | tmux is optional for humans, not a
runtime dependency | cassettes are versioned evidence artifacts

## Cassette Data Contract

Every cassette is a single versioned directory or archive with a manifest and
append-only event streams. Version `1` contains:

| Field | Required | Description |
|-------|----------|-------------|
| `manifest.version` | Yes | Cassette schema version. Starts at `1`; incompatible changes increment it. |
| `manifest.harness` | Yes | Harness name, binary path fingerprint, binary version string when available, and capability row snapshot. |
| `manifest.command` | Yes | Scrubbed argv, working directory policy, environment allowlist names, timeout settings, and permission mode. |
| `manifest.terminal` | Yes | Initial rows/cols, resize events, locale, TERM value, and PTY mode flags needed for replay. |
| `manifest.provenance` | Yes | Agent git SHA, contract version, OS/arch, recorded-at timestamp, and recorder version. |
| `input.jsonl` | Yes | User/input events: bytes sent to stdin, paste boundaries, control keys, resize events, signal events, and timing deltas. |
| `output.raw` | Yes | Raw output bytes from the PTY, exactly as observed after environment scrubbing. |
| `frames.jsonl` | Yes | Screen snapshots or frame diffs at normalized timestamps for human review and deterministic replay assertions. |
| `service-events.jsonl` | Yes | CONTRACT-003 service events emitted during the run, including routing, tool, final, and typed-drain-compatible payloads. |
| `final.json` | Yes | Exit status, signal, duration, final metadata, usage, cost, routing actual, session log path, and normalized final text. |
| `quota.json` | When applicable | Scrubbed quota/status probe output and parsed quota windows used to accept or reject the record run. |
| `scrub-report.json` | Yes | Redaction rules applied, environment values removed, secret-pattern hit counts, and fields intentionally preserved. |

Timing is stored as monotonic deltas from cassette start. Replay may scale or
collapse delays, but it must preserve event order, resize ordering, process
exit, and final service metadata.

## Record Mode

Record mode runs the real harness binary through the direct PTY transport. It
fails before writing a cassette when:

- the harness binary is missing or not executable;
- authentication is missing, expired, or for the wrong account;
- subscription or quota state cannot be confirmed for subscription harnesses;
- requested model, reasoning, permission, or workdir capability is unsupported
  by the harness capability matrix;
- the run exits before producing a final service event.

If a failure happens after cassette creation starts, the recorder writes an
explicit failed-run artifact only under a diagnostic path, never as accepted
golden-master evidence.

## Replay Mode

Replay mode never uses credentials and never contacts a provider. It feeds the
recorded input/output/frame streams through the same parser, service-event
decoder, and typed drain assertions used by live mode. Replay can prove parser,
event-shape, cancellation, cleanup, and PTY transport behavior; it cannot prove
that a live external harness still works today.

Replay is deterministic by default:

- timestamps are interpreted as ordered deltas, not wall-clock requirements;
- environment is reconstructed only from the cassette allowlist;
- terminal size and resize events come from `manifest.terminal`;
- service-event assertions compare typed payloads after documented scrub rules,
  not raw secrets or machine-specific paths.

## Inspection

Live inspection attaches to a read-only mirror of the PTY stream. Inspectors may
watch frames and output bytes but cannot write to stdin, resize the authoritative
PTY, or mutate cassette files. Recorded-run inspection reads `frames.jsonl` and
`output.raw` through a viewer that opens files read-only and never normalizes or
rewrites the evidence.

## Alternatives

| Option | Pros | Cons | Evaluation |
|--------|------|------|------------|
| **Direct PTY ownership in agent** | One dependency-light lifecycle for execution, record, replay, cancellation, and inspection; portable test seams; cassette format can be shaped around CONTRACT-003 events | Requires careful PTY implementation and platform testing; attach UX must be built | **Selected: best fit for a library-first service boundary without a global tmux dependency** |
| Standardize on tmux for all harness lifecycle | Mature attach/detach UX, pane capture, process supervision already exists | Makes tmux a hard dependency for library consumers and CI; Windows portability is poor; machine-local tmux state complicates deterministic replay | Rejected: acceptable as a human debugging adapter, not as the canonical service transport |
| Keep tmux only for quota/status while direct exec handles normal runs | Minimal short-term change | Violates the single-transport concern; quota behavior and live execution would diverge; cassette replay could not prove the path that quota probes use | Rejected: partial helper is explicitly the failure mode this ADR resolves |
| Adopt ntm or another terminal recorder as the core | Potentially faster to get visual snapshots | Adds another lifecycle owner without CONTRACT-003 semantics; uncertain process cleanup, service-event capture, and quota integration | Rejected: useful reference material, but not the service boundary |
| Split a generic PTY cassette project now | Clean abstraction if multiple projects need it | Premature API freeze; no second consumer yet; slows harness support beads | Rejected for now; revisit after one stable format and a second consumer |

## Consequences

| Type | Impact |
|------|--------|
| Positive | Harness execution, quota probes, record/replay, cancellation, and inspection share one PTY lifecycle. |
| Positive | Library consumers do not need tmux installed to use or test DDX Agent harness support. |
| Positive | Golden-master cassettes can carry CONTRACT-003 service events and typed-drain payloads as first-class evidence. |
| Negative | The project must own PTY edge cases: resize races, process groups, signal handling, terminal modes, and OS portability. |
| Negative | Read-only inspection needs a purpose-built viewer instead of relying on `tmux attach`. |
| Neutral | tmux may still be used manually by developers outside the service contract, but it is not required evidence. |

## Risks

| Risk | Prob | Impact | Mitigation |
|------|------|--------|------------|
| Direct PTY implementation leaks subprocesses on cancellation | M | H | Add process-group cleanup tests, timeout tests, and failed-run diagnostics before marking live capabilities supported |
| Cassette scrub rules remove data needed for replay | M | M | Store scrub reports and compare replay against typed events rather than raw secrets |
| Replay creates false confidence about live harness availability | H | M | Keep live-run policy: fresh record-mode evidence is required to promote or retain `supported` capability status |
| Cross-platform PTY behavior diverges | M | M | Define OS-specific transport adapters behind one cassette contract and require per-OS fixtures before claiming support |

## Validation

| Success Metric | Review Trigger |
|----------------|----------------|
| A future cassette runner can record and replay one codex or claude run through the same direct PTY transport | Record and replay use different process/session supervisors |
| Accepted cassettes contain manifest, input, output, frames, service events, final metadata, quota data when applicable, and scrub report | A cassette lacks any required version-1 artifact |
| Record mode refuses missing auth/quota/binary cases before writing accepted evidence | CI or local record mode creates a passing cassette for an unauthenticated harness |
| Inspection cannot alter the live PTY or recorded files | Viewer writes to stdin, resizes the authoritative PTY, or rewrites cassette artifacts |

## Concern Impact

- **Resolves inspectable harness execution concern**: Selects direct PTY
  ownership as the single architecture and rejects tmux as a required partial
  helper.
- **Supports harness capability matrix**: Future `supported` harness
  capabilities can cite versioned cassette evidence produced by this transport.

## References

- [CONTRACT-003 DdxAgent Service Interface](/Users/erik/Projects/agent/docs/helix/02-design/contracts/CONTRACT-003-ddx-agent-service.md)
- [Concerns](/Users/erik/Projects/agent/docs/helix/01-frame/concerns.md)
- [Architecture](/Users/erik/Projects/agent/docs/helix/02-design/architecture.md)

## Review Checklist

- [x] Context names a specific problem
- [x] Decision statement is actionable
- [x] At least two alternatives were evaluated
- [x] Each alternative has concrete pros and cons
- [x] Selected option's rationale explains why it wins
- [x] Consequences include positive and negative impacts
- [x] Negative consequences have mitigations
- [x] Risks are specific with probability and impact assessments
- [x] Validation section defines review triggers
- [x] Concern impact is complete
- [x] ADR is consistent with governing feature spec and PRD requirements
