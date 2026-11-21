# Terminal, agents and CI

Run local verification with the working tree and policy you are developing.

```sh
itervyn verify --config itervyn.json --format json
```

Keep the JSON and process exit separately. A required native FAIL dominates other
provider observations; required missing/unusable evidence is INCONCLUSIVE unless
a required failure is already known. Optional outcomes warn. An Itervyn integrity
or publication failure is ERROR. PASS is emitted only after the run is committed.
Inspect a known run without executing anything:

```sh
itervyn inspect RUN_ID --config itervyn.json --format json
```

## Evaris lifecycle

Evaris validates an experiment, freezes inputs in prepare, accepts externally
executed results through import, then evaluates the imported run. Itervyn owns
only the final evaluate call, using an explicit store/run ID:

```sh
evaris validate --experiment experiment.json
evaris prepare --experiment experiment.json --tasks-root /absolute/new/tasks
evaris import --store /absolute/store --record /absolute/import.json
```

External execution and the native import request must follow Evaris's documented
contracts. Do not substitute arbitrary cwd tests, an experiment filename, or an
agent's success claim for an imported run. The Itervyn adapter calls
`evaris evaluate --store ABS --run ID` and creates a fresh immutable evaluation.
It does not select latest or overwrite evaluations. Native report generation also
writes records and is not part of Itervyn inspection.

There is no `--allow-commands` or `--invaris` forwarding. Required command graders
and required embedded verification remain unavailable/inconclusive. That
restriction avoids implicitly executing commands or duplicating direct Invaris
checks. Optional native errors remain visible while retaining the native
assessment. To verify a changed output, prepare/import a new native run and update
the explicit runId. Itervyn records imported and live subjects separately and
does not attest equality between them.

## Agent instruction template

The following can be added to project instructions read by Codex or Claude Code,
or used alongside Panda Suite's existing execute/test/review instructions:

> Check whether Itervyn is installed and read `itervyn --help` and `itervyn version`
> before using commands. Preserve the task's acceptance criteria. After relevant
> implementation and development tests, run `itervyn verify --config itervyn.json
> --format json`. Read its status, every required provider, blockers, evidence
> references and scope. Report completion only when applicable acceptance criteria
> and independently observed verification evidence support it. On FAIL, investigate
> and correct the defect, then reverify. On INCONCLUSIVE or ERROR, investigate
> prerequisites or stop with the blocker; never fabricate evaluation outcomes.
> Do not disable/weaken checks, rewrite evidence or substitute a stale imported run
> to obtain PASS. Limit the agent correction loop to 3 attempts, then report the
> remaining blocker and evidence. Itervyn itself does not retry or edit source.

This is portable instruction guidance, not proof of enforcement or an interactive
host integration test. Panda Lead and Craft remain optional. No new skill, MCP
server, agent execution API or Panda Suite modification is required.

## CI enforcement

The [revision gate workflow](../.github/workflows/revision-gate.yml) checks
the GitHub event's exact candidate SHA. On `pull_request`, this is GitHub's
synthetic test merge of the PR with its base. On `merge_group`, it is the
queue's integration commit, which can also include earlier queued changes.
For manual dispatch it is the chosen ref's event commit. The PR branch head
is useful for feedback on that branch alone. Require the merge queue if the
gate must run on the current integration candidate even when the target
branch advances. Merge methods can produce different eventual commit IDs.
The result attests to the SHA it actually checked.
[GitHub Actions events](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows)
[GitHub merge queue](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue)

The workflow builds Itervyn and the import guard from a separate checkout of
the maintainer-selected policy SHA. It builds Invaris from a pinned release
commit. The candidate is a fresh, disposable checkout with no persisted
checkout credentials and no build outputs. The trusted launcher puts the one
staged harness at its reserved path, adds the exact path to the disposable
checkout's Git exclude file, and removes the staged file after verification.
Existing files at the reserved path are refused. Keep generated input and
output out of this checkout. Git reports other untracked and ignored files
as dirty under the gate.

The launcher runs the trusted verifier, provider and guard in an unprivileged
container with a cleared environment, a read-only root filesystem and
read-only subject, policy and binary mounts. The container has no network or
CI credentials. The guard parses Go imports from `internal/verify` and rejects
imports under Itervyn's canonical provider adapter namespace without executing candidate code.
The existing compatibility workflow runs candidate tests separately. The gate
uploads the verdict, event, candidate and policy IDs, and native provider release and revision. Raw streams and
native evidence stay in the private evidence directory during the job.

### Maintainer setup

1. Review and land a bootstrap commit containing the workflow, launcher,
   verifier, harness and guard. Approve that full SHA on a protected ref and
   set the repository Actions variable `ITERVYN_TRUSTED_POLICY_SHA` to it.
   Keep the gate optional until this commit exists and a protected run
   succeeds. A missing or invalid pin fails the job. Update the pin only after
   independently reviewing future policy commits.
2. Require this workflow from a protected source using an organization or
   enterprise ruleset, or run the equivalent check from independent CI and
   require that check from its trusted integration. The job name is
   `Verify integration candidate`. A status name alone can be reproduced by
   an editable workflow from the same GitHub Actions app. Protect the source
   workflow, approved policy ref and variable administration. Apply the rule
   to the merge target. Configure code-owner review for policy and workflow
   changes, restrict bypass and direct pushes, and require the merge queue
   if gating on queue commits.
3. Exercise a normal PR and a merge-group run. Check the artifact SHA against
   the integration SHA and confirm a fixture that adds a forbidden import
   and weakens candidate policy still fails. Enable the rule after those
   checks. Keep the provider compatibility job required separately if desired.

[GitHub ruleset workflow rules](https://docs.github.com/en/enterprise-cloud@latest/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets#require-workflows-to-pass-before-merging)

## Compatibility

Itervyn accepts a provider binary only when all of these hold:

1. It is a regular executable file, not a symbolic link or script launcher.
2. Its Go build identity names the provider's module
   (`github.com/akkaraponph/invaris` or `github.com/akkaraponph/evaris`) at a
   clean, tagged release `vX.Y.Z` whose `X.Y` is a supported release line.
   Development builds (`(devel)`), pseudo-versions, prereleases and modified
   trees fail closed.
3. `<tool> --version` exits 0 and prints exactly `<tool> vX.Y.Z`, the same
   version as the build identity.

Anything else is `UNSUPPORTED_TOOL`; a missing or linked executable is
`TOOL_MISSING`. The executable's SHA-256 is pinned between identification and
execution, and recorded with each run.

| Itervyn | Tool | Release | `--version` | Contract read | Status |
| --- | --- | --- | --- | --- | --- |
| 0.1.x | Invaris | 0.5.x | `invaris v0.5.Z` | `invaris.dev/v1alpha2` check harness, `invaris.evidence/v1alpha2` evidence | Latest tested: v0.5.1 |
| 0.1.x | Evaris | 0.4.x or 0.5.x | `evaris v0.4.Z` or `evaris v0.5.Z` | `evaris.evaluation/v1alpha1` of an existing imported run | Latest tested: v0.5.0 |
| 0.1.x (this branch) | either | any other release line, development build or pseudo-version | any | not checked | Refused, `UNSUPPORTED_TOOL` |
| 0.1.0 | Invaris | v0.3.0 / `d770885` | none | `invaris.evidence/v1alpha2` without `line` | Legacy; refused by this branch |
| 0.1.0 | Evaris | v0.2.0 / `e12566b` | `evaris v0.2.0` | `evaris.evaluation/v1alpha1` without verification `message` | Legacy; refused by this branch |

Itervyn CI resolves the latest stable GitHub release of each tool on every run,
including a daily scheduled check. It tests the resolved tags as a pair and
fails if a new release falls outside the supported profiles. Use
`itervyn doctor --format json` to see the locally installed versions. Update
the provider binaries separately when you want to use a newer verified release.

Itervyn reads `evaris.evaluation/v1alpha1` only. Evaris reports
(`evaris.report/v1alpha2`) and runs (`evaris.run/v1alpha2`) stay inside the
Evaris store and are not decoded. Decoding is strict: an unknown field or a newer
schema version is refused rather than ignored, so widening `SupportedReleases`
in `internal/process/process.go` needs a contract review and new real-binary
tests against that release. No store migrations or mixed-schema fallback are
performed.
