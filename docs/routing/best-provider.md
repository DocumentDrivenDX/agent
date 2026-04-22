# Best-Provider Contract

This contract defines how route resolution chooses the best candidate from the
known `(harness, provider, endpoint, model)` tuples.

## Baseline Definition

The baseline contract applies after explicit harness, model, and provider pins
have narrowed the candidate set and after the requested profile or model ref has
resolved to a concrete model for each harness surface.

1. Available candidates outrank unavailable candidates. A subprocess harness is
   available when its binary is present. Native provider endpoints are available
   only when the configured endpoint is usable; live-discovery providers must
   return models from their `/v1/models`-compatible endpoint.
2. Quota OK candidates outrank quota-exhausted candidates. Quota-tracked
   subscription harnesses require fresh usable quota or auth evidence.
   Pay-per-token and local endpoints are not quota-constrained by this gate.
3. Capability and compatibility gates must pass: requested context window, tool
   support, reasoning level, permissions, model allow-list, exact-pin support,
   profile surface mapping, and live model discovery.
4. Lowest known cost wins among candidates that meet the availability, quota,
   capability, and compatibility gates. Cost is the estimated blended USD cost
   per 1,000 tokens. Unknown cost is neutral: it uses the average of known
   eligible costs for the tie-break. If all costs are unknown, the cost
   tie-break is skipped.
5. Locality breaks remaining ties. A local cost class is preferred when cost is
   equal or all costs are unknown.
6. Names make the final order deterministic. Harness name sorts first, then
   provider name.

Profile scoring is not a baseline step ahead of cost. It enters only through the
profile-specific ordering overrides below.

The returned decision includes the full ranked candidate trace, including
ineligible candidates and rejection reasons.

## Profile Overrides

- `local`, `offline`, and `air-gapped`: add a `local-only` hard constraint
  before the baseline availability step. Non-local candidates are ineligible,
  and explicit pins that can only be served by non-local harnesses return
  `ErrProfilePinConflict`. Remaining local candidates use the baseline cost,
  locality, and deterministic-name order.
- `smart`, `code-smart`, and `code-high`: add a `subscription-only` hard
  constraint before the baseline availability step and reorder capability/profile
  score ahead of raw cost among subscription candidates. Subscription candidates
  with OK quota receive profile preference; cost remains the next tie-break
  after capability/profile score. Local harness pins conflict with the profile.
- `cheap` and `code-economy`: keep local-first fallback semantics but override
  ordering with the strongest cost-class penalty and local bonus. This makes
  local and lower-cost economy candidates rank ahead of higher-cost candidates
  unless availability, quota, capability, cooldown, or reliability signals make
  them non-viable or lower-scored. Exact known cost still breaks ties within the
  profile score.
- `default`, `standard`, `fast`, `code-fast`, and `code-medium`: keep
  local-first fallback semantics with balanced profile score. The score may
  account for local preference, quota health, cooldowns, reliability, provider
  affinity, observed speed, and latency before exact known cost breaks remaining
  ties.

## Engine Gates And Score Signals

The routing engine exposes these additional candidate signals in the trace and
uses them inside the profile overrides above:

- Capability gates: requested context window, tool support, reasoning level,
  permissions, model allow-list, exact-pin support, profile surface mapping,
  and live model discovery.
- Profile score signals: profile policy, provider preference, quota
  freshness/trend, observed success, cooldowns, provider affinity, observed
  speed, and observed latency.

## Cost And Quota Notes

Subscription harness cost is an effective cost derived from catalog model price
and quota pressure. Healthy quota can make subscription cost effectively free;
near-exhausted quota raises the effective cost and applies quota score
penalties. Stale quota does not always reject a candidate by itself, but it is
demoted unless the subscription routing decision marks the harness unusable.

Local endpoint cost is unknown unless the caller supplies
`LocalCostUSDPer1kTokens`. Unknown local cost remains neutral in the explicit
cost tie-break, while the profile overrides above can still add local-first
score signals before that tie-break.
