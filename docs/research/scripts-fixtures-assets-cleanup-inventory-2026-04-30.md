---
ddx:
  id: scripts-fixtures-assets-cleanup-inventory-2026-04-30
  bead: agent-6543053e
  parent: agent-996dca04
  criteria: cleanup-criteria-2026-04-30
  created: 2026-04-30
---

# CL-004 scripts, fixtures, and assets cleanup inventory

This is an inventory artifact only. No scripts, fixtures, cast files, JSONL
sessions, benchmark profiles, generated assets, or `testdata/` entries are
deleted or moved by CL-004.

The classification rules come from
`docs/research/cleanup-criteria-2026-04-30.md`: active script references, code
readers, tests, website inclusion, or documented historical/non-delete roles are
`keep`; stale historical artifacts with no active references are `archive`;
delete requires stronger no-reference and no-history evidence. This pass found
no direct `delete` candidates.

## Evidence commands

The rows below cite these repository checks, all run from the repository root on
2026-04-30:

- `find scripts bench demos eval testdata internal website/static tests -type f \( -name '*.sh' -o -name '*.py' -o -name '*.yaml' -o -name '*.yml' -o -name '*.json' -o -name '*.jsonl' -o -name '*.cast' -o -name '*.env' -o -path '*/testdata/*' \) | sort` -> 86 files in the explicit CL-004 file-extension scope.
- `find . -path './.git' -prune -o -path './.ddx/executions' -prune -o -type f \( -path './scripts/*' -o -path './bench/*' -o -path './demos/*' -o -path './eval/*' -o -path './testdata/*' -o -path './internal/*/testdata/*' -o -path './website/static/*' -o -path './tests/*' \) -print | wc -l` -> 109 tracked files in the broader scripts/fixtures/assets/testdata scope.
- `go test ./...` -> PASS.
- `go test ./cmd/bench ./internal/benchmark/profile ./internal/comparison ./internal/harnesses/codex ./internal/harnesses/claude ./internal/harnesses/gemini ./internal/ptytest ./eval/navigation ./internal/corpus ./internal/modelcatalog` -> PASS.
- `go test ./agentcli` -> PASS.
- `python3 scripts/beadbench/test_run_beadbench.py` -> 21 tests PASS.
- `python3 scripts/beadbench/test_probe_reasoning_controls.py` -> 2 tests PASS.
- `python3 -m unittest discover -s scripts/benchmark/harness_adapters -p 'test_*.py'` -> 12 tests PASS.
- `python3 -m pytest scripts/benchmark/harness_adapters` -> blocked locally because Python has no `pytest` module; the equivalent stdlib `unittest` command above is the runnable local test surface.
- `bash -n scripts/benchmark/legacy-4tool-baseline.env scripts/benchmark/evidence-grade-comparison.env scripts/benchmark/run_benchmark.sh scripts/benchmark/smoke_run.sh demos/record.sh tests/install_sh_acceptance.sh` -> PASS.

## Inventory

| id | path | type | classification | reason | evidence | superseder / replacement | follow-up |
|---|---|---|---|---|---|---|---|
| `CL-004.01` | `scripts/coverage-ratchet.go` | script-go | keep | Active Makefile quality gate. | `rg -n 'scripts/coverage-ratchet.go' Makefile` -> `coverage-ratchet`, `coverage-bump`, and `coverage-trend` invoke `go run scripts/coverage-ratchet.go`; `go test ./...` PASS. | None. | FZ rename updates command/product strings only if present. |
| `CL-004.02` | `install.sh`, `tests/install_sh_acceptance.sh` | install-script | keep | Public installer and its acceptance test are active project surfaces. | `rg -n 'install.sh|install_sh_acceptance' README.md CONTRIBUTING.md tests Makefile docs scripts` shows the test script and documented installer mentions; `bash -n tests/install_sh_acceptance.sh` PASS; `tests/install_sh_acceptance.sh` is the targeted dry-run surface for installer edits. | None. | Rename bead updates binary/module names in place. |
| `CL-004.03` | `scripts/beadbench/run_beadbench.py`, `scripts/beadbench/test_run_beadbench.py`, `scripts/beadbench/manifest-v1.json` | benchmark-script | keep | Current Beadbench runner, tests, and manifest. | `rg -n 'run_beadbench|manifest-v1' scripts/beadbench docs CHANGELOG.md` shows README, tests, and research-note references; `python3 scripts/beadbench/test_run_beadbench.py` -> 21 tests PASS. | None. | Rename bead updates old product/CLI names in script output and docs examples. |
| `CL-004.04` | `scripts/beadbench/corpus.yaml`, `scripts/beadbench/corpus/*.yaml` | benchmark-fixtures | keep | Active corpus index and per-bead fixtures. | `agentcli/corpus_commands.go:18-20` pins `corpusRoot = "scripts/beadbench"`; `internal/corpus/corpus.go` loads `corpus.yaml` and per-bead YAML; `go test ./agentcli` PASS; `go test ./internal/corpus` included in targeted Go PASS. | None. | None for cleanup. |
| `CL-004.05` | `scripts/beadbench/probe_reasoning_controls.py`, `scripts/beadbench/test_probe_reasoning_controls.py` | benchmark-script | keep | Active reasoning-control probe with local tests and research references. | `rg -n 'probe_reasoning_controls.py' scripts/beadbench docs` shows README and research references; `python3 scripts/beadbench/test_probe_reasoning_controls.py` -> 2 tests PASS. | None. | None. |
| `CL-004.06` | `scripts/beadbench/external/termbench-subset.json`, `scripts/beadbench/external/termbench-subset-canary.json` | benchmark-fixture | keep | Active external Terminal-Bench subset manifests. | `cmd/bench/external_termbench.go:106` defaults to `termbench-subset.json`; `cmd/bench/matrix.go:25` defaults matrix runs to `termbench-subset-canary.json`; `cmd/bench/external_termbench_test.go:14` opens the canary file; targeted `go test ./cmd/bench` PASS. | None. | None. |
| `CL-004.07` | `scripts/benchmark/run_benchmark.sh`, `scripts/benchmark/smoke_run.sh`, `scripts/benchmark/harbor_agent.py`, `scripts/benchmark/evidence-grade-comparison.env` | benchmark-script | keep | Current Terminal-Bench/Harbor runner surface and evidence-grade config. | `scripts/benchmark/README.md` documents both scripts, `cmd/bench/external_termbench.go:255` points operators to `scripts/benchmark/run_benchmark.sh`, `internal/benchmark/external/termbench/doc.go:31` names `harbor_agent.py`; `bash -n` on both shell scripts and the env file PASS. | None. | Rename bead updates `ddx-agent` binary examples after productinfo decisions land. |
| `CL-004.08` | `scripts/benchmark/task-subset-v2.yaml` | benchmark-fixture | keep | Current real-ID evidence-grade comparison subset. | `scripts/benchmark/run_benchmark.sh:31` defaults `DDX_BENCH_SUBSET_FILE` to `task-subset-v2.yaml`; `scripts/benchmark/evidence-grade-comparison.env:18` exports it; `docs/helix/02-design/solution-designs/SD-009-benchmark-mode.md` says v2 is the correct before/after subset. | None. | None. |
| `CL-004.09` | `scripts/benchmark/task-subset-v1.yaml` | benchmark-fixture | keep | Historical placeholder subset, but explicitly documented as retained. | `scripts/benchmark/README.md:98-99` says v2 is active and v1 remains as the historical placeholder manifest; `docs/helix/02-design/solution-designs/SD-009-benchmark-mode.md:178-179,233-234` records the same retention role. Active references mean it is not an archive candidate in CL-004. | `scripts/benchmark/task-subset-v2.yaml` for current benchmark runs. | Future retention-policy bead may archive v1 only after docs references are updated. |
| `CL-004.10` | `scripts/benchmark/legacy-4tool-baseline.env` | benchmark-profile-env | archive | Historical baseline config has no active callers and sits in an active benchmark directory. | `rg -n 'legacy-4tool-baseline\.env|legacy-4tool-baseline' scripts Makefile lefthook.yml docs README.md CONTRIBUTING.md website cmd internal agentcli eval tests bench demos` -> no matches outside the file itself; no `.github/workflows` caller found; `bash -n scripts/benchmark/legacy-4tool-baseline.env` PASS; the file points to current `task-subset-v2.yaml` but no script sources it. | No replacement; `scripts/benchmark/evidence-grade-comparison.env` is the active comparable env file. | CL-007 may move this to a clearly marked research archive, or delete it if a follow-up proves no historical retention value. |
| `CL-004.11` | `scripts/benchmark/profiles/gpt-5-3-mini.yaml`, `scripts/benchmark/profiles/noop.yaml`, `scripts/benchmark/profiles/smoke.yaml` | benchmark-profiles | keep | Active matrix profile fixtures loaded by CLI and tests. | `cmd/bench/profiles.go:12` defaults to `scripts/benchmark/profiles`; `internal/benchmark/profile/profile_test.go` loads all three profile files; targeted `go test ./internal/benchmark/profile` PASS; `docs/research/model-census-2026-04-29.md` cites `gpt-5-3-mini.yaml`. | None. | Rename bead updates provider/product strings only if needed. |
| `CL-004.12` | `scripts/benchmark/harness_adapters/**` | benchmark-adapters | keep | Active multi-harness benchmark adapter set with local tests. | `docs/helix/02-design/solution-designs/SD-010-harness-matrix-benchmark.md:216,222,265` defines this adapter directory and test fake; `docs/research/harness-matrix-plan-2026-04-29.md:184-198,497-498` lists the adapter split; `python3 -m unittest discover -s scripts/benchmark/harness_adapters -p 'test_*.py'` -> 12 tests PASS. | None. | None. |
| `CL-004.13` | `bench/corpus/*.yaml` | benchmark-corpus | keep | Active built-in benchmark corpus. | `cmd/bench/runner.go:173` defaults corpus to `bench/corpus`; `cmd/bench/bench_test.go:11-19` walks to `bench/corpus`; `bench/README.md:16` documents each YAML task; targeted `go test ./cmd/bench` PASS. | None. | None. |
| `CL-004.14` | `eval/navigation/fixtures.yaml` | eval-fixture | keep | Active navigation micro-eval fixture. | `eval/navigation/eval_test.go` loads and validates navigation fixtures; `docs/helix/02-design/epic-validation-e8c1f21c.md:59` cites the fixture; targeted `go test ./eval/navigation` PASS. | None. | None. |
| `CL-004.15` | `eval/tasktracking/fixtures.yaml` | eval-fixture | archive | Detailed historical fixture with no package or loader in the current tree. | `rg -n 'tasktracking|eval/tasktracking|Task Tracking|task tracking compliance' .` -> only the file itself and `docs/helix/02-design/epic-validation-e8c1f21c.md:60`; `go test ./eval/tasktracking/...` -> `matched no packages` / `no packages to test`; no Go, Python, shell, or CLI loader references this basename. | No replacement. | CL-007 may move to a research archive with a note that the intended runner never landed, or file a follow-up if task-tracking evals should be revived. |
| `CL-004.16` | `demos/record.sh`, `demos/scripts/demo-*.sh`, `website/static/demos/*.cast` | demo-scripts-assets | keep | Demo generator scripts and website-rendered cast assets are active. | `website/content/demos/_index.md:12,29,45` embeds all three `.cast` files; `website/content/demos/_index.md:62` links `demos/scripts/` and `demos/record.sh`; `README.md:52` links `website/static/demos/file-read.cast`; `bash -n demos/record.sh` PASS. | None. | Rename bead updates website path/product strings in place. |
| `CL-004.17` | `demos/sessions/demo-read.jsonl`, `demos/sessions/demo-edit.jsonl`, `demos/sessions/demo-bash.jsonl` | demo-session-jsonl | archive | Raw session captures have no active consumer; published casts are the website assets. | `rg -n 'demo-read.jsonl|demo-edit.jsonl|demo-bash.jsonl' . --glob '!demos/sessions/*'` -> no matches; broader `rg -n 'demos/sessions' README.md CONTRIBUTING.md docs website demos scripts Makefile cmd internal agentcli tests bench` -> only `demos/record.sh` comments and copy target; no test or CI replay loader exists. | Website assets in `website/static/demos/*.cast` are the active published demos. | CL-007 may move these raw captures to an archive or delete them if no replay workflow is added. |
| `CL-004.18` | `testdata/harness-cassettes/{claude,codex}/**` | pty-cassettes | keep | Active harness golden replay fixtures. | `harness_golden_integration_test.go:23` sets `harnessCassetteRoot = "testdata/harness-cassettes"` and `:226` opens the `claude`/`codex` cassette dirs; `docs/helix/02-design/harness-golden-integration.md:8` defines the suite; targeted non-integration Go run excludes build-tagged test, but the fixture-loader references are direct. | None. | None. |
| `CL-004.19` | `testdata/harness-cassettes/live/{claude,codex}/**` | live-pty-cassettes | keep | Manual/live replay fixtures are documented and env-selectable. | `harness_golden_integration_test.go:281,347` reads `FIZEAU_HARNESS_CASSETTE_DIR`; `internal/harnesses/codex/quota_pty_integration_test.go:51` and `internal/harnesses/claude/quota_pty_integration_test.go:54` also honor the env var; `docs/helix/02-design/harness-golden-integration.md:53,61` documents `FIZEAU_HARNESS_CASSETTE_DIR=./testdata/harness-cassettes/live`. | None. | None. |
| `CL-004.20` | `internal/harnesses/{claude,codex,gemini}/testdata/**` | harness-testdata | keep | Active harness parser and quota usage fixtures. | `internal/harnesses/codex/runner_test.go:304,415` loads `tool_events.jsonl` and `usage_cassettes/*.jsonl`; `internal/harnesses/claude/stream_test.go:269` loads `usage_cassettes/*.jsonl`; `internal/harnesses/gemini/runner_test.go:324,381` loads `live-ok-20260421.jsonl` and `gemini-cli-0.38.2-models.txt`; targeted harness Go tests PASS. | None. | None. |
| `CL-004.21` | `internal/ptytest/testdata/docker-conformance/*.{sh,yaml}` | ptytest-fixtures | keep | Active PTY conformance scenario fixtures. | `internal/ptytest/docker_conformance_integration_test.go:589` locates `testdata/docker-conformance`; `internal/ptytest/scenario.go:111` reads scenario YAML; `docs/helix/02-design/harness-golden-integration.md:190` cites the fixtures; targeted `go test ./internal/ptytest` PASS. | None. | None. |
| `CL-004.22` | `internal/comparison/testdata/benchmark-feat019.json` | comparison-fixture | keep | Active comparison benchmark fixture. | `internal/comparison/benchmark_test.go:12` loads `testdata/benchmark-feat019.json`; targeted `go test ./internal/comparison` PASS. | None. | None. |
| `CL-004.23` | `internal/modelcatalog/catalog/models.yaml` | generated-catalog-asset | keep | Embedded canonical model catalog snapshot. | `internal/modelcatalog/manifest.go:27-28` uses `//go:embed catalog/models.yaml`; `Makefile:14` passes it to `catalogdist`; `docs/helix/02-design/plan-2026-04-10-catalog-distribution-and-refresh.md:44,327` declares it canonical; targeted `go test ./internal/modelcatalog` PASS. | None. | Rename bead updates config path/product naming through catalog surfaces, not cleanup. |

## Summary for CL-007

Archive candidates:

- `CL-004.10` — `scripts/benchmark/legacy-4tool-baseline.env`
- `CL-004.15` — `eval/tasktracking/fixtures.yaml`
- `CL-004.17` — `demos/sessions/*.jsonl`

Delete candidates: none from this inventory pass.

All other rows classify as `keep` because they have active script references,
code loaders, targeted tests, website inclusion, or explicit documented
historical-retention references. The Fizeau rename should update those files in
place rather than remove them.
