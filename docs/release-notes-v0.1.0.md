# Itervyn v0.1.0 release notes (provisional, local)

Status: provisional. Not committed, tagged, pushed or published. Based on worktree on top of `3ab92f3a7628e6a0242de07116c6a35f9675dbec`, with uncommitted changes (25 modified, 7 untracked files).

## Summary
- Local CLI `itervyn` with `init`, `doctor`, `verify`, `inspect`, `version`.
- Verification aggregates Invaris (native check profile `invaris.dev/v1alpha2`) and Evaris (imported-run evaluation) into fail-closed results.
- New trusted configuration `itervyn.config/v1alpha2` with external config, harness, provider and guard programs, and independent evidence destination.
- New revision gate: `--expect-revision <sha>` (and `project.requireCleanTree` in v1alpha2 policy) records Git state before and after providers, compares it with native Invaris provenance, and blocks PASS on missing, dirty, moved or conflicting provenance. Live Invaris evidence is required by the gate; Evaris alone cannot satisfy it.
- New result schema `itervyn.result/v1alpha2` (gated results). v1alpha1 config and result remain supported.
- CI revision-gate workflow (`.github/workflows/revision-gate.yml`) with read-only candidate mounts, cleared environment, trusted policy from an independent commit; a trusted AST guard rejects forbidden provider imports in `internal/verify`.
- Launcher fix from the Panda implementation (`.itervyn-policy/run-gate.sh`): refuses pre-existing reserved harness file; stages exact trusted harness bytes.

## Compatibility
- Additive. Existing `itervyn.config/v1alpha1` and `itervyn.result/v1alpha1` are accepted unchanged.
- Unknown schema versions or fields are refused (per `docs/release.md`).
- Provider support: Invaris v0.5.1 and Evaris v0.5.0 (latest stable at validation); Evaris v0.4.0 compatibility subset passed in earlier validation.
- No evidence-store migration is promised (`docs/release.md`).

## Known limits (see readiness)
- Gate runs on a writable local filesystem have no same-user isolation.
- The gate's trusted container can write evidence; future candidate-code execution needs a separate boundary.
- No Evaris-to-candidate revision linkage is claimed.
- Hosted GitHub pull-request and merge-queue runs require maintainer bootstrap and a protected ruleset (`docs/integration.md`); not performed.

## Upgrade / rollback
- Replace the binary; completed records are not rewritten. Old binaries reject unsupported records.
- Evaris evaluations created before a failure are retained; no automatic deletion.
