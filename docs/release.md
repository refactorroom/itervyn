# v0.1.0 distribution and release gates

This repository implements a local CLI, not a publishing service. Releases require
separate maintainer authorization. Local packaging does not tag, commit or push.

| Target | Support |
| --- | --- |
| darwin/arm64, darwin/amd64 | macOS binaries |
| linux/arm64, linux/amd64 | Linux binaries |
| Windows | Excluded; Unix process-group behavior is required |

Go minimum is 1.27.4. Use the same Go version and source for reproducibility:

```sh
python3 scripts/package.py --go /absolute/selected/go
```

The script builds every target twice with CGO disabled, trimpath, VCS stamping
disabled and an empty build ID, compares exact binary bytes, then writes
deterministic tar.gz archives and SHA256SUMS under ignored `dist/`. Version
`v0.1.0` and owned schema versions are compiled into the CLI. Each archive contains
the binary, license, guides, schemas and examples. No private run outputs belong
in an archive. Review SHA256SUMS and rehearse extraction plus `itervyn version`
before any authorized publication. Cross-building alone does not prove native
execution, especially on the other CPU architecture.

Local configuration and results support `/v1alpha1`. The trusted configuration and revision gate result use `/v1alpha2`. Diagnostic and doctor schemas use `/v1alpha1`. Unknown versions
or fields are refused. No 0.x cross-minor compatibility or evidence-store migration
is promised. Supported provider releases and profiles are documented in
[integration](integration.md). Upgrading a provider or evidence shape requires
new positive and negative real-binary checks before broadening accepted builds.

Release gates:

1. Public contracts and requirement acceptance tests pass; strict JSON and exits
   agree with [configuration](configuration.md).
2. Linux CI tests the latest stable GitHub releases together. For release
   preparation, also check the supported older releases and native macOS
   execution locally. Skipped tests do not count as evidence.
3. Native process-group, environment, output, path and integrity tests pass and
   focused review has no unresolved high-impact execution/evidence defect.
4. Corruption, collision, interrupted publication, canceled/partial runs and
   read-only inspection meet the documented guarantees.
5. Four packages build reproducibly; fresh install/quick start, guide links,
   license provenance and actual source revision are reviewed.

A completed implementation or green unit suite does not by itself pass every
release gate. Record the actual source revision, working-tree diff, tool hashes,
commands, platform, native versus cross-built coverage and remaining limitations.
No benchmark, real-model quality gain, interactive agent-host test or hosted CI
success follows from authored fixtures or a local workflow definition.

Rollback replaces the binary without rewriting completed records. Old binaries
reject unsupported records. Evaris evaluations created before an Itervyn failure
are retained; there is no automatic deletion or store rollback. Trusted guard
side effects outside the evidence directory cannot be undone by Itervyn.
