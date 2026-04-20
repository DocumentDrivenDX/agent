# Project Concerns — DDX Agent

## Area Labels

| Area | Description |
|------|-------------|
| `all` | Every bead |
| `lib` | Core library packages (agent loop, tools, providers, logging) |
| `cli` | Standalone CLI binary |

## Active Concerns

- **go-std** — Go + Standard Toolchain (areas: all)
- **testing** — Multi-layer testing with property-based, fuzz, and E2E coverage (areas: all)

## Project Overrides

### go-std

- **CLI framework**: None. DDX Agent CLI is minimal enough for `flag` stdlib. Cobra
  is not needed.
- **Test framework**: Use `testing` stdlib + `testify/assert` for assertions.
  No external test runner.
- **Structured logging**: Use `log/slog` from stdlib. No third-party logger.
- **HTTP client**: Use provider SDKs (`openai-go`, `anthropic-sdk-go`) directly.
  No custom HTTP client abstraction.

### testing

- **Property-based testing**: Use `pgregory.net/rapid` for property-based tests
  in Go. Define properties for all serialization (session log events),
  tool-call round-trips, and provider message translation.
- **Fuzz testing**: Use Go's native `testing.F` fuzz support for parsers,
  config loading, and tool input handling.
- **E2E testing**: Full agent loop E2E tests run against LM Studio with a
  loaded model (build tag `e2e`). Verify a complete file-read-and-edit
  workflow end-to-end.
- **Integration tests**: Provider integration tests against real LM Studio and
  real Anthropic API using build tags (`integration`, `e2e`).
- **Harness integration evidence**: A harness is not considered supported by
  tests unless at least one real-binary integration test invokes the installed
  harness through the public DDX Agent surface (`DdxAgent.Execute` or the
  standalone CLI path). Parser tests, fixture replay, and fake binaries are
  unit/contract evidence only; they must not be described as proving Claude
  Code, OpenAI Codex, Pi, OpenCode, or any other external harness works.
- **Deterministic harness tests**: When deterministic behavior is needed, use a
  reusable virtual/deterministic harness with dictionary-driven prompts,
  stable events, stable token usage, and explicit slow/error/cancel modes.
  This harness verifies shared service infrastructure and event contracts, not
  external harness compatibility.
- **Harness capability matrix**: Every harness capability must be declared in a
  machine-checkable matrix with one of: `required`, `supported`,
  `unsupported`, or `experimental`. Every `supported` capability requires real
  integration evidence for that harness. `unsupported` capabilities must not be
  advertised by the public API. `experimental` capabilities are excluded from
  "fully supported" claims until promoted and covered by integration evidence.
- **Harness capability granularity**: Do not collapse distinct harness behavior
  into vague labels. Track default model reporting, exact model pinning,
  declared/catalog model support, live model discovery, reasoning selection,
  progress events, token usage, session logging, cancellation, permission mode
  handling, workdir/context use, and quota monitoring as separate capabilities.
- **Quota tests**: Claude Code and OpenAI Codex quota monitoring requires
  parser, cache, public API, and real quota-probe integration coverage. Tests
  should prove probe/cache/API behavior without requiring measurable quota burn;
  before/after quota-consumption deltas are manual or optional unless they can
  be made cheap, stable, and account-safe.
- **Test data**: Use `rapid` generators for structured test data (Messages,
  ToolCalls, TokenUsage). Factory functions with sensible defaults for complex
  types.
- **Performance ratchets**: Track agent loop overhead (<1ms per iteration
  excluding model inference) and tool execution overhead via benchmarks.
