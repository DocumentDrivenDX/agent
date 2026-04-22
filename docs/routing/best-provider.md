# Best-Provider Contract

This contract defines how route resolution chooses the best candidate from the
known `(harness, provider, endpoint, model)` tuples.

## Baseline Definition

1. Apply hard profile constraints before normal eligibility. `local-only`
   profiles keep only local candidates; `subscription-only` profiles keep only
   subscription candidates. Contradictory explicit pins return
   `ErrProfilePinConflict`.
2. Available candidates outrank unavailable candidates. A subprocess harness is
   available when its binary is present. Native provider endpoints are available
   only when the configured endpoint is usable; live-discovery providers must
   return models from their `/v1/models`-compatible endpoint.
3. Quota OK candidates outrank quota-exhausted candidates. Quota-tracked
   subscription harnesses require fresh usable quota or auth evidence.
   Pay-per-token and local endpoints are not quota-constrained by this gate.
4. Capability gates must pass: requested context window, tool support,
   reasoning level, permissions, model allow-list, exact-pin support, profile
   surface mapping, and live model discovery.
5. Profile score ranks eligible candidates. The score incorporates profile
   policy, provider preference, quota freshness/trend, observed success,
   cooldowns, provider affinity, observed speed, and observed latency.
6. Lowest known cost wins among candidates with equal profile score. Cost is the
   estimated blended USD cost per 1,000 tokens. Unknown cost is neutral: it uses
   the average of known eligible costs for the tie-break. If all costs are
   unknown, the cost tie-break is skipped.
7. Locality breaks remaining ties. A local cost class is preferred when cost is
   equal or all costs are unknown.
8. Names make the final order deterministic. Harness name sorts first, then
   provider name.

The returned decision includes the full ranked candidate trace, including
ineligible candidates and rejection reasons.

## Profile Overrides

- `local`, `offline`, and `air-gapped`: add a `local-only` hard constraint
  before availability checks. Non-local candidates are ineligible, and explicit
  pins that can only be served by non-local harnesses return
  `ErrProfilePinConflict`.
- `smart`, `code-smart`, and `code-high`: add a subscription-only hard
  constraint and rank capability before raw cost by targeting `code-high`.
  Subscription candidates with OK quota receive profile preference. Local
  harness pins conflict with the profile.
- `cheap` and `code-economy`: target `code-economy`, give local candidates the
  largest local bonus, and apply the steepest cost-class penalty. Lower cost is
  favored before capability when scores are otherwise close.
- `default`, `standard`, `fast`, `code-fast`, and `code-medium`: target
  `code-medium`, prefer local endpoints, and apply a balanced cost-class
  penalty. These profiles are cost-aware but can use subscription harnesses when
  they remain eligible and rank best.

## Cost And Quota Notes

Subscription harness cost is an effective cost derived from catalog model price
and quota pressure. Healthy quota can make subscription cost effectively free;
near-exhausted quota raises the effective cost and applies quota score
penalties. Stale quota does not always reject a candidate by itself, but it is
demoted unless the subscription routing decision marks the harness unusable.

Local endpoint cost is unknown unless the caller supplies
`LocalCostUSDPer1kTokens`. Unknown local cost remains neutral in the explicit
cost tie-break, but local profiles and local-first preferences can still add
score before that tie-break.
