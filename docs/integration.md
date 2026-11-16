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

## Compatibility pins

| Tool | Tested integration target | Native contract |
| --- | --- | --- |
| Invaris | v0.3.0 / d770885fd33012d9ff368b25611f8b327d348b6e | native v1alpha2 check harness/evidence only |
| Evaris | v0.2.0 / e12566ba22fa2d63fe3ecb68ed61bf3dbd38b065 | v1alpha1 evaluation of an existing imported run |

Build identity must contain the expected module and release version or clean
pinned VCS revision. A version string alone is not evidence of a passing native
operation. New schema shapes/profiles need explicit compatibility work. Evaris's
embedded Invaris support was pinned upstream to an older source revision; matching
the v1alpha2 label does not establish interchangeability with newer shapes.
No store migrations or mixed-schema fallback are performed.
