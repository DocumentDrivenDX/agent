---
ddx:
  id: cleanup-criteria-2026-04-30
  bead: agent-235efb9a
  parent: agent-996dca04
  created: 2026-04-30
---

# Pre-rename cleanup criteria — CL-000 epic

This doc defines the objective criteria the CL-002 / CL-003 / CL-004 inventory
beads use to classify candidates, the evidence the CL-005 / CL-006 / CL-007
deletion beads must cite, and the rollback expectations for any deletion or
archival landed before the Fizeau rename.

The goal of CL-000 is not to clean up the codebase in general — it is to
shrink the rename surface so FZ-001 operates on the smallest practical set of
files. Anything outside that scope is out-of-scope for this epic.

## Decision: delete / archive / keep

Every candidate identified by an inventory bead **must** be assigned exactly
one of three classifications. The classification determines what the
corresponding deletion bead is allowed to do.

### Delete

A candidate may be classified `delete` only if **all** of the following hold:

1. **No active references.** No production code, test, build script, CI
   workflow, generated asset, docs page, or CLI path imports, includes,
   embeds, or links to it.
2. **No external entry point.** It is not a public API surface, exported
   symbol used by an external consumer (binaries built from `cmd/`,
   published packages, install-script targets), or documented user-facing
   command.
3. **No load-bearing history role.** It is not the canonical source for a
   shipped behavior whose history future maintainers will need to read
   (see "Historical artifacts" below — those are `keep`, not `delete`).
4. **Either dead or duplicated.** It is one of:
   - **Dead**: no callers, no readers, no consumers (orphan code, dangling
     fixture, stale cast file, obsolete profile, generated artifact whose
     generator is gone).
   - **Duplicated**: another file/package supersedes it, the supersession
     is documented (commit message, ADR, prior research note), and no
     references still point at the old version.

Any candidate that meets `delete` should be removed outright — `git`
history preserves it, so archival adds noise without value.

### Archive

A candidate is `archive` if it has **historical value** but **no active
references**. Archive means: move into a clearly-marked archival location
(e.g. `docs/research/archive/`, `docs/helix/<phase>/archive/`), not delete.

Archive when:

- The artifact records a decision, experiment, or baseline that future
  readers will plausibly need to consult, **and**
- It has no active references (no one links to it, imports it, or runs
  it), **and**
- Leaving it in the active tree would mislead a reader into thinking it is
  current.

If an artifact has active references, fix the references first or
classify it as `keep` — do not archive something that is still linked to.

### Keep

A candidate is `keep` if any of the following hold:

- It has at least one active reference (import, link, CLI invocation,
  generated-asset dependency, test fixture lookup).
- It is in a non-delete category (see next section).
- The inventory cannot conclusively prove it is dead within the time
  budget of the inventory bead. **Default to `keep` when uncertain** —
  the rename is the deadline-driven work, not the cleanup.

A `keep` classification with old naming is fine; renames will pick those
up. The point of CL-000 is only to shrink the surface, not to scrub it.

## Non-delete categories (explicit)

The following categories are **never** classified `delete` or `archive` by
the CL beads, even if they reference old naming or look stale. They are
either historical (touch only with a separate, well-justified bead) or
they belong to a different epic:

| Category | Why it is `keep` here |
|---|---|
| `CHANGELOG.md` entries | Historical record of shipped behavior; renames are appended, never rewritten. |
| `docs/helix/**` retrospectives, ADRs, design notes for shipped work | HELIX phase artifacts — historical decisions, not active docs. May reference old names. |
| Git-tracked release notes, milestone summaries | Same: historical. |
| `.ddx/beads.jsonl`, `.ddx/executions/**` | DDx execution evidence; not source. |
| Third-party vendored code (if any appears under `internal/` or via submodules) | Out of scope for CL-000. |
| Files that are touched only by FZ-* beads (the rename itself) | The rename handles them; CL beads must not. |
| Anything flagged by an open bead outside `area:rename, kind:cleanup` | Belongs to that bead's epic. |

If an inventory bead believes one of these categories should be revisited,
it must file a follow-up bead rather than reclassify it.

## Required evidence by candidate type

Each row in an inventory bead's table **must** cite the evidence
appropriate to its candidate type. Inventories without this evidence are
not acceptable input for the deletion beads.

### Go code (CL-002 → CL-005)

For every Go package, file, exported symbol, or sub-package proposed for
deletion, the inventory must cite all of:

| Evidence | Concrete form |
|---|---|
| Package builds and tests today | `go list ./...` includes it; `go test ./...` passes on `HEAD`. |
| No importers in this repo | `rg -n '"<full/import/path>"'` returns no results outside the package itself, or `go list -f '{{.ImportPath}}: {{.Imports}}' ./... \| rg <pkg>` is empty. |
| No symbol references | `rg -n '\b<ExportedSymbol>\b'` returns no callers outside the package's own files. |
| Not a binary entry point | Not under `cmd/<name>`, not referenced by `Makefile`, not referenced by `install.sh` or any CI workflow. |
| Tests still pass after a local removal trial | `go test ./...` and `go vet ./...` pass with the candidate removed (record the command and result; do not commit the trial). |
| Lint clean | `golangci-lint run ./...` (or whichever lint Makefile uses) passes after trial removal. |

For duplicate supersession, additionally cite:

- The superseding package/file path.
- The commit, ADR, or research note documenting the supersession.
- `rg` evidence that all references now point at the superseder.

### Docs (CL-003 → CL-006)

For every doc file, page, or section proposed for deletion or archival,
the inventory must cite:

| Evidence | Concrete form |
|---|---|
| Inbound link search | `rg -n '<filename>' docs/ README.md AGENTS.md CONTRIBUTING.md website/` returns the expected set (typically: only the file itself and known historical mentions). |
| External link search | `rg -n '<filename>'` across the whole repo returns no hits in code, scripts, CI, or generated-site sources outside the docs tree. |
| Website inclusion check | If the docs are rendered (e.g. `website/`, Hugo content), confirm whether the file is built into the site (`rg <filename> website/content/` etc.). A built page is `keep` until the site stops including it. |
| Classification | `active` (current docs), `historical` (HELIX/changelog/ADR), or `superseded` (point at the replacement). |
| For `superseded` candidates: replacement | File path or URL of the doc that replaces it; `rg` evidence that consumers now reference the replacement. |

`historical` always classifies as `keep`. Only `superseded` (with a
replacement) and `active-but-stale-and-orphaned` (no inbound links, no
website inclusion, no replacement needed because it documented a removed
behavior) classify as `delete` or `archive`.

### Scripts, fixtures, cast files, sessions, profiles, assets, testdata (CL-004 → CL-007)

For every script, fixture, cast file, JSONL session, benchmark profile,
generated asset, or `testdata/` entry proposed for deletion, the
inventory must cite:

| Evidence | Concrete form |
|---|---|
| No script references | `rg -n '<basename>' scripts/ Makefile lefthook.yml .github/` returns no callers. |
| No code references | `rg -n '<basename>'` across `*.go`, `*.ts`, `*.js`, `*.py` returns no readers. |
| No CI references | `rg -n '<basename>' .github/workflows/` is empty. |
| No fixture-loader reference | For `testdata/` entries: `rg -n '<basename>' .` shows no test loads it (`os.Open`, `embed.FS`, `filepath.Join("testdata", …)`). |
| Not regenerable from a live generator | If the file is generated, the generator either no longer exists or the regenerated artifact is not consumed. Cite the generator path or its absence. |
| Targeted re-run passes | After local removal trial: relevant targeted test or script dry-run passes. Cite the exact command (e.g. `go test ./internal/benchmark/...`, `bash scripts/beadbench/<x>.sh --dry-run`). |

Cast files, JSONL session captures, and old benchmark profiles are
typical candidates: they often outlive the experiment that produced them.
Verify whether a research note, doc, or test still references them before
classifying.

## Inventory table template

CL-002, CL-003, and CL-004 should each produce a table of this shape (or
several tables, one per area, if the candidate count warrants it). The
deletion beads (CL-005, CL-006, CL-007) cite the row by its `id` when
landing the change.

| id | path | type | classification | reason | evidence | superseder / replacement | follow-up |
|---|---|---|---|---|---|---|---|
| `CL-002.01` | `internal/foo/bar.go` | go-file | delete | no importers; symbol `Bar` unreferenced | `go list ./... ` clean; `rg '"…/internal/foo"'` empty; `rg '\bBar\b'` empty; `go test ./...` green after trial removal | — | — |
| `CL-003.07` | `docs/research/old-thing.md` | docs-research | archive | superseded by `docs/research/new-thing-2026-04-15.md`; one stale inbound link | `rg 'old-thing.md' docs/ website/` → only the stale link; that link is in a doc already marked obsolete | `docs/research/new-thing-2026-04-15.md` | move to `docs/research/archive/` and drop the stale inbound link in same commit |
| `CL-004.12` | `testdata/harness-cassettes/legacy-foo.json` | fixture | delete | no test loads it; cassette format superseded | `rg 'legacy-foo.json' .` empty; `rg 'harness-cassettes' .` shows only loaders for current cassettes | — | — |

Column rules:

- `id`: stable across the epic. Inventory beads assign these and they are
  cited verbatim by the deletion beads' commit messages.
- `classification`: exactly one of `delete`, `archive`, `keep`.
- `reason`: one short clause per row. If a `keep` row exists, give the
  category (e.g. "historical: HELIX retrospective", "active: imported by
  cmd/agentcli").
- `evidence`: list the actual commands and their outputs in summary form
  (e.g. `rg "<pkg>" empty`, `go test ./... pass`). The deletion bead
  re-runs these and cites them in its commit.
- `superseder / replacement`: required for any `superseded` row.
- `follow-up`: any side-effect the deletion bead must make in the same
  commit (drop a stale link, update an index, regenerate a `go.sum`).

## Per-deletion checklist (used by CL-005 / CL-006 / CL-007)

Before staging a deletion, the implementing bead **must** confirm each
item against the candidate row:

- [ ] Inventory row exists, is classified `delete` or `archive`, and is
      cited by the bead's commit message.
- [ ] Re-run the row's evidence commands on `HEAD`; results still match.
      If they do not, requeue the row to inventory rather than deleting.
- [ ] No new references have appeared since the inventory landed
      (`rg` on the symbol/path/basename one more time).
- [ ] For Go: `go test ./...`, `go vet ./...`, lint pass after the
      deletion is staged.
- [ ] For docs: site build (if applicable) and any docs-link checks pass;
      no inbound link is left dangling.
- [ ] For scripts/fixtures/assets: targeted tests or dry-runs that touch
      the affected area pass.
- [ ] Commit stages only the intended files (`git add <paths>`, never
      `-A`), and the commit message subject ends with the candidate
      `id` from the inventory table.
- [ ] One commit per area split (the deletion beads' descriptions
      already require splitting by package/area; honor that).

A deletion bead that cannot tick every box should classify the candidate
back to `keep` and document the blocker rather than ship a partial
deletion.

## Rollback expectations

Cleanup deletions must be cheaply reversible until the rename lands:

1. **One commit per area, narrowly scoped.** Each deletion commit touches
   only one area (one Go package, one docs subtree, one fixture
   directory). A `git revert <sha>` on a single commit must be sufficient
   to bring the area back exactly as it was. No drive-by edits in
   deletion commits.
2. **No mixed concerns.** Cleanup commits do not perform renames, do not
   refactor, and do not rewrite unrelated docs. If a deletion exposes a
   refactor opportunity, file a follow-up bead.
3. **Pre-revert verification on `HEAD`.** The deletion-bead commit
   message records the exact verification commands it ran (`go test
   ./...`, lint, doc build, targeted script). On revert, those same
   commands are the regression check.
4. **Window for revert.** Deletions are revertible until CL-008 publishes
   the post-cleanup rename-surface count. Anything reverted after that
   point requires re-running CL-008.
5. **Archive is a `git mv`.** Archive commits use `git mv` so history is
   preserved across the move. A revert is `git mv` back; no content
   change.
6. **No deletions during the rename.** Once FZ-001 starts, the CL epic is
   closed. Late-discovered dead code is filed as a separate bead, not
   absorbed into a rename commit.
7. **Hard backstop.** If any post-deletion failure surfaces in CI, on a
   contributor's machine, or via a missing-fixture test failure, the
   default response is `git revert` first and re-classify the row to
   `keep` second. Do not patch forward under time pressure during the
   pre-rename window.

## Out of scope

CL-000 explicitly does **not** cover:

- Renaming, repackaging, or moving live code.
- Refactors, even ones that would obviously help the rename.
- Editing historical HELIX or changelog artifacts to use new names.
- Cleaning up third-party submodules or vendored trees.
- Deleting issues, beads, or execution evidence under `.ddx/`.

Anything that requires those is FZ work or a separate epic.
