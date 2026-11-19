# Terminal, agents and CI

Use the same command at every boundary:

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

Provision approved tools independently of agent-controlled outputs, then make the
same verify command a required CI step. Nonzero status fails the job:

```yaml
- name: Verification
  run: itervyn verify --config itervyn.json --format json
```

Protect the workflow, trusted tool provisioning and accepted configuration through
repository controls; a prompt or a check in a writable task repository cannot
enforce itself against an actor able to remove it. Preserve private run output in
an access-controlled CI artifact when required. Avoid publishing raw streams.
An Evaris store must be prepared/imported before this step if Evaris is required.

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
