# Routing Profile Catalog

The embedded model catalog is the source of truth for profile names. This page
documents the profile catalog added by agent-191a74f9 and currently loaded from
`internal/modelcatalog/catalog/models.yaml` (`catalog_version: 2026-04-12.3`).

`ProviderPreference` values:

- `local-first`: prefer eligible local endpoints, with subscription fallback
  when the profile has no hard local-only constraint.
- `local-only`: reject non-local candidates before ranking.
- `subscription-first`: prefer quota-backed subscription harnesses, with profile
  constraints deciding whether local fallback is allowed.
- `subscription-only`: reject non-subscription candidates before ranking.

## Catalog Profiles

### `air-gapped`

- Name: `air-gapped`
- Intent: Keep routing inside an air-gapped or no-network execution boundary.
- Target: `code-economy`
- ProviderPreference: `local-only`
- Expected cost class: local
- Example use case: Running DDx queue work in an environment where cloud
  harnesses are forbidden.
- Hard constraints: Only local candidates are eligible; non-local harness or
  cloud-only model pins return `ErrProfilePinConflict`.

### `cheap`

- Name: `cheap`
- Intent: Minimize spend for simple coding work while retaining fallback
  options.
- Target: `code-economy`
- ProviderPreference: `local-first`
- Expected cost class: local or cheap
- Example use case: Bulk mechanical edits, formatting fixes, and low-risk
  queue work.
- Hard constraints: None beyond normal candidate availability, capability, and
  pin compatibility gates.

### `code-economy`

- Name: `code-economy`
- Intent: Select the canonical economy coding tier directly.
- Target: `code-economy`
- ProviderPreference: `local-first`
- Expected cost class: local or cheap
- Example use case: Calling the economy target explicitly from tooling that
  does not use alias names.
- Hard constraints: None beyond normal candidate availability, capability, and
  pin compatibility gates.

### `code-fast`

- Name: `code-fast`
- Intent: Preserve the older fast-profile name while targeting the medium coding
  tier.
- Target: `code-medium`
- ProviderPreference: `local-first`
- Expected cost class: local or medium
- Example use case: Existing consumers that still request `code-fast`.
- Hard constraints: None beyond normal candidate availability, capability, and
  pin compatibility gates.

### `code-high`

- Name: `code-high`
- Intent: Select the canonical high-capability coding tier directly.
- Target: `code-high`
- ProviderPreference: `subscription-first`
- Expected cost class: quota-backed medium or expensive subscription
- Example use case: Hard implementation, design, or debugging tasks that should
  start on frontier subscription harnesses.
- Hard constraints: Subscription candidates only; local harness or local-only
  model pins return `ErrProfilePinConflict`.

### `code-medium`

- Name: `code-medium`
- Intent: Select the canonical balanced coding tier directly.
- Target: `code-medium`
- ProviderPreference: `local-first`
- Expected cost class: local or medium
- Example use case: Tooling that wants the standard model tier without using
  human-facing profile names.
- Hard constraints: None beyond normal candidate availability, capability, and
  pin compatibility gates.

### `code-smart`

- Name: `code-smart`
- Intent: Preserve the older smart-profile name while targeting the high coding
  tier.
- Target: `code-high`
- ProviderPreference: `subscription-first`
- Expected cost class: quota-backed medium or expensive subscription
- Example use case: Existing consumers that still request `code-smart`.
- Hard constraints: Subscription candidates only; local harness or local-only
  model pins return `ErrProfilePinConflict`.

### `default`

- Name: `default`
- Intent: Provide balanced routing with local endpoints preferred before
  subscription fallback.
- Target: `code-medium`
- ProviderPreference: `local-first`
- Expected cost class: local or medium
- Example use case: Unattended DDx work where predictable cost control is more
  important than maximum capability.
- Hard constraints: None beyond normal candidate availability, capability, and
  pin compatibility gates.

### `fast`

- Name: `fast`
- Intent: Favor the medium coding tier for responsive interactive work.
- Target: `code-medium`
- ProviderPreference: `local-first`
- Expected cost class: local or medium
- Example use case: Short interactive coding prompts where latency matters.
- Hard constraints: None beyond normal candidate availability, capability, and
  pin compatibility gates.

### `local`

- Name: `local`
- Intent: Require local endpoint routing and forbid subscription upgrades.
- Target: `code-economy`
- ProviderPreference: `local-only`
- Expected cost class: local
- Example use case: Private-code runs, reproducible local tests, or development
  with LM Studio, Ollama, or OMLX.
- Hard constraints: Only local candidates are eligible; non-local harness or
  cloud-only model pins return `ErrProfilePinConflict`.

### `offline`

- Name: `offline`
- Intent: Require local endpoint routing for offline automation.
- Target: `code-economy`
- ProviderPreference: `local-only`
- Expected cost class: local
- Example use case: Running an agent without reachable subscription harnesses.
- Hard constraints: Only local candidates are eligible; non-local harness or
  cloud-only model pins return `ErrProfilePinConflict`.

### `smart`

- Name: `smart`
- Intent: Route difficult work to the high-capability coding tier.
- Target: `code-high`
- ProviderPreference: `subscription-first`
- Expected cost class: quota-backed medium or expensive subscription
- Example use case: Complex feature work, cross-file debugging, architecture
  decisions, or review tasks that need stronger reasoning.
- Hard constraints: Subscription candidates only; local harness or local-only
  model pins return `ErrProfilePinConflict`.

### `standard`

- Name: `standard`
- Intent: Use the balanced medium coding tier with local-first cost control.
- Target: `code-medium`
- ProviderPreference: `local-first`
- Expected cost class: local or medium
- Example use case: Day-to-day implementation where cost and reliability should
  outweigh maximum model capability.
- Hard constraints: None beyond normal candidate availability, capability, and
  pin compatibility gates.
