# Configuration and output contracts

Configuration is strict JSON. The [local config schema](../schemas/config-v1alpha1.schema.json)
uses project-root policy. The [trusted config schema](../schemas/config-v1alpha2.schema.json)
supports selecting policy outside the project. The Go parser also checks recursive duplicate keys (including escaped
equivalents), exact field spelling, paths, provider combinations and bounds.
Unknown fields, invalid UTF-8, nulls, trailing JSON and more than 1 MiB are rejected.
Parsing and path resolution start no process. Omission selects a documented
default; explicit null never selects a default.

| Field | Omitted default | Accepted values |
| --- | --- | --- |
| schemaVersion | Required | `itervyn.config/v1alpha1` or `itervyn.config/v1alpha2` |
| project.root | `.` | Existing directory, relative to the config directory or absolute, without `..` components |
| project.requireCleanTree | `false` | Boolean in v1alpha2. Activates the revision gate even when the expected commit flag is missing. |
| execution.timeoutSeconds | 300 | Integer 1–3600, shared pipeline deadline |
| execution.providerTimeoutSeconds | 120 | Integer 1–3600, capped by remaining pipeline time |
| execution.stdoutLimitBytes | 4194304 | Integer 1–4194304 per provider |
| execution.stderrLimitBytes | 65536 | Integer 1–65536 per provider |
| execution.environmentAllowlist | `[]` | Unique environment identifier names; PATH/TMPDIR reserved |
| providers | Required | Fixed `invaris` and `evaris` members; omitted member disabled |
| providers.NAME.enabled | Required in explicit member | Boolean |
| providers.NAME.required | Required in explicit member | Boolean; disabled+required invalid |
| providers.NAME.executable | Provider name | Bare PATH name or absolute regular executable file; no relative slash paths |
| providers.invaris.action | `check` | Only `check` |
| providers.invaris.harness | `invaris.json` | Regular file. A project-root basename in v1alpha1, or a safe path relative to the config directory in v1alpha2. |
| providers.evaris.store | Required when enabled | Explicit absolute native store |
| providers.evaris.runId | Required when enabled | 32 lowercase hexadecimal characters; no latest selector |
| evidence.directory | `.itervyn/runs` | Project-relative, or absolute in v1alpha2. The revision gate requires an absolute path outside the project. |

At least one enabled provider must be required. Empty paths/executables, empty
explicit provider objects, fractional limits, integer overflow and duplicate
allowlist names are errors. Empty allowlist `[]` passes no additional variables.
A missing allowlisted environment variable stays absent. Evaris store/run fields
must be omitted when it is disabled; supplying empty strings is also invalid.
Missing native files/tools are runtime prerequisites and cannot become PASS.
Evidence paths must avoid links, traversal and overlap with either harness or the Evaris store.

`itervyn init` emits an enabled, required Invaris provider and disabled Evaris.
It does not generate guards or overwrite existing config. To add Evaris after
preparation/import, add its actual store/run context, for example:

```json
{
  "schemaVersion": "itervyn.config/v1alpha1",
  "providers": {
    "invaris": {"enabled": true, "required": true},
    "evaris": {
      "enabled": true,
      "required": false,
      "executable": "evaris",
      "store": "/absolute/private/evaris-store",
      "runId": "0123456789abcdef0123456789abcdef"
    }
  }
}
```

Replace both example values with actual committed native identifiers; the example
does not create a store or establish a passing evaluation. The fields are
Itervyn-owned configuration translated to native `--store` and `--run` flags.

## Supported Invaris harness

Use native `invaris.dev/v1alpha2`, trusted-local/unrestricted; nonempty explicit
policies with `allowEmpty:false`; nonempty structured command guards with known
policyId; every guard exactly once in requiredGuards; requiredProofs empty;
failure-first precedence; allowWeakEvidence false. No steps, challenges,
adapters, requiredAdapters or provider-backed guards. The
[example harness](../examples/go-project/invaris.json) shows the profile.
Native optional command arguments, environment names, 0–300 second guard limits
and positive minInspected remain native configuration. Unsupported actions and
v1alpha1/v1alpha4 harnesses are refused before execution. Itervyn never rewrites
a harness to make it acceptable.

## Revision gate

Supply `itervyn verify --config /trusted/itervyn.json --expect-revision SHA`
with a full lowercase Git commit ID. This flag activates all revision checks.
Set `project.requireCleanTree: true` in v1alpha2 policy to make omitting the
flag a recorded blocker. The gate uses the candidate repository root and
samples Git before and after providers. It requires both observations to
report the expected commit, with no tracked, untracked or ignored file changes.
An index using skip-worktree or assume-unchanged is refused. Invaris must
produce live evidence for the same clean commit. An Invaris check is effective
as required in this mode. A separately established required failure retains its
FAIL verdict when provenance has issues.

In v1alpha2, the trusted launcher makes the exact harness bytes available at
`.invaris/itervyn-harness.json` inside the candidate. Invaris 0.5 derives the
target and relative guard arguments from the harness directory or the parent
of `.invaris`. Its native target, loaded config digest and normalized policy
must match the requested subject and trusted input. The gate filters only
this one launcher-provided, ignored path from Git status. A tracked entry at
that path fails the cleanliness check. The launcher rejects pre-existing
content at the reserved path before staging and keeps it read-only while
Invaris runs. See [the CI launcher and bootstrap](integration.md#ci-enforcement).

The gate requires external trusted config, harness, regular absolute provider
and guard executables, plus out-of-tree evidence. Provider and guard environment
allowlists must be empty. Invaris sensitive-output storage must be disabled.
The launcher controls the OS environment and tool installation. Imported
Evaris results keep their own scope and can serve as additional requirements.
An imported PASS by itself never verifies the candidate.

In strict Invaris checks, diagnostics at report, guard and observation level
block PASS. The native 0.5 check contract uses them for failed, refused,
skipped or incomplete work and has no diagnostic severity field. Compatibility
warnings and truncation of a retained observation excerpt are informational.
An expected exit-1 `COMMAND_FAILED` diagnostic on a validated structured FAIL
is part of the failure. For required Evaris evaluations, pending review,
execution cancellation, missing or contradictory required grader/verification
outcomes prevent PASS. Optional criterion reasons and import limitations remain
advisory. Ungated runs retain the legacy aggregate.

## Output

`--format json` emits one object; no progress text is mixed into stdout. Default
format is text. `version` reports version and supported schema lists. `init`
reports created/configPath/warnings; `doctor` emits
`itervyn.doctor/v1alpha1`. Ungated verification uses
[itervyn.result/v1alpha1](../schemas/result-v1alpha1.schema.json). The revision gate
uses [itervyn.result/v1alpha2](../schemas/result-v1alpha2.schema.json) and adds the
expected commit, before/after Git observations, native Invaris provenance,
policy separation and provider evidence completeness. Inspection reads both
versions. Preflight and usage failures use
[itervyn.diagnostic/v1alpha1](../schemas/diagnostic-v1alpha1.schema.json).

All result fields are required; unknown values are explicit null and lists are
non-null arrays. The reserved taskId is always null. Provider nativeDecision,
nativeSchemaVersion, exitCode, nativeReasons, executable hash and scope are
separate from normalized status. Evaris scope preserves preparation, imported
run, evaluation, experiment, case and arm identity plus run/oracle digests.
Feedback is a safe summary; original diagnostics remain in private native JSON.

Evidence artifacts use `sha256:<64 lowercase hex>` over exact persisted bytes.
Every artifact reference includes safe relative path and byte size. result.json
lists artifacts; COMMITTED hashes result.json, avoiding a self-hash cycle.
Raw native evidence may contain secrets, so a valid schema does not make it safe
to share.

Exit 0 means PASS, 1 means trustworthy required FAIL, 2 means INCONCLUSIVE,
ERROR, usage, readiness or inspection failure. Code 2 requires reading the JSON
status/code. inspect returns the stored verification exit on success. No provider
exit is blindly forwarded. CLI arguments reject unknown/repeated flags, empty
values and extra positional arguments. `--help` is available without tool lookup.
