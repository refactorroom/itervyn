# Itervyn v0.1.0 release readiness (local, provisional)

Readiness: **BLOCKED** for publication. Local preparation is complete; hosted and exact-revision gates are open.

## Revision and worktree
- Base: `3ab92f3a7628e6a0242de07116c6a35f9675dbec` (no tags exist; first release candidate).
- Worktree: dirty. 25 modified and 7 untracked files (see `git status`). Nothing committed.
- Therefore no evidence below is for a committed revision. It applies to the dirty worktree as observed at the time noted.

## Version decision
- Provisional `v0.1.0`. Basis: `docs/release.md` target, no existing tags, no evidence of prior distribution in this repository.
- Uncertainty: prior distribution outside this repository is unknown. If v0.1.0 was distributed elsewhere, bump to v0.2.0 (owner decision).

## Evidence (observed now, dirty worktree)
| Check | Command | Result |
| --- | --- | --- |
| Formatting | `gofmt -l cmd internal examples .itervyn-policy/guard` | clean |
| Build and vet | `go build ./...`, `go vet ./...`, vet of `.itervyn-policy/guard` | pass |
| Unit tests | `go test -count=1 ./...` | pass |
| Real providers with race | `ITERVYN_TEST_INVARIS=<invaris v0.5.1> ITERVYN_TEST_EVARIS=<evaris v0.5.0> go test -race ./... -count=1` | pass (all packages) |
| Whitespace | `git diff --check` | clean |
| Schemas against prior records | `scripts/validate-schemas.py --evidence <prior evidence-final>` | 6 schemas, 20 pairs valid |
| Packaging | `python3 scripts/package.py --go /opt/homebrew/Cellar/go/1.27.4/libexec/bin/go` | all four targets reproducible |
| Native archive smoke | extract `darwin_arm64`, run `itervyn version --format json` | version v0.1.0, schemas as listed |

Provider binaries: invaris v0.5.1 (sha256 prefix `98ddc227…`), evaris v0.5.0 (`e3d15be6…`), from validation directory built by Panda. Provenance of these binaries was not independently re-verified here.

Earlier evidence (not reproduced by this pass): Evaris v0.4.0 compatibility subset (sha prefix `2128dc2f…`); Docker gate fixture cases (clean PASS; ignored and reserved file INCONCLUSIVE; forbidden import with weakened policy FAIL; write probe). These came from an earlier worktree state.

Not run: native macOS execution beyond the archive smoke test; older supported Invaris releases; Linux archive execution (no Linux host here).

## Packaging artifacts (local, ignored `dist/`)
- `itervyn_v0.1.0_darwin_arm64.tar.gz` sha256 `e60255f4…`
- `itervyn_v0.1.0_darwin_amd64.tar.gz` sha256 `b41b4fe8…`
- `itervyn_v0.1.0_linux_arm64.tar.gz` sha256 `3e358c79…`
- `itervyn_v0.1.0_linux_amd64.tar.gz` sha256 `474ddc23…`
- `SHA256SUMS` lists all four.

## Gates
| Gate | Status | Blocks publication? |
| --- | --- | --- |
| Commit of the change set | Open (no commit) | Yes |
| Exact-revision evidence (after commit) | Open; rerun the test, schema and package commands on the committed revision | Yes |
| Hosted CI (`ci.yml`, `revision-gate.yml`) | Not run; needs maintainer bootstrap and protected ruleset | Yes |
| Older supported provider releases | Partial (Evaris v0.4.0 earlier); Invaris older not checked | Yes (release gate 2) |
| Native macOS and Linux execution | macOS arm64 archive smoke only | Yes (release gate 2) |
| Owner acceptance of v0.1.0 vs v0.2.0 | Open | Yes |
| Release notes and readiness review | Drafted here | Review needed |

## Checklist (proposed, not executed)
1. Owner decides version (v0.1.0 provisional).
2. Owner reviews the 32-file change set and commits it on a branch (not by Taro).
3. On the committed revision: `gofmt`, `go build`, `go vet`, `go test -race ./...` with real providers, schema validation, `scripts/package.py`.
4. Maintainer completes hosted bootstrap and protected ruleset per `docs/integration.md`; run hosted CI on the PR.
5. Native execution checks on macOS arm64 and amd64 and Linux; extract and run `itervyn version` for each archive.
6. Verify SHA256SUMS against the committed revision's rebuilt archives.
7. Owner-authorized tag and publication (not performed here).

## Rollback
- Replace the binary; old binaries reject unsupported records. Evaris store is not rolled back automatically.
- Trusted guard side effects outside the evidence directory cannot be undone by Itervyn.
