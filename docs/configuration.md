# Configuration and output contracts

Configuration is JSON with `schemaVersion: "itervyn.config/v1alpha1"`. The
[published config schema](../schemas/config-v1alpha1.schema.json) describes its
shape. The Go parser also checks recursive duplicate keys (including escaped
equivalents), exact field spelling, paths, provider combinations and bounds.
Unknown fields, invalid UTF-8, nulls, trailing JSON and more than 1 MiB are rejected.
Parsing and path resolution start no process. Omission selects a documented
default; explicit null never selects a default.

| Field | Omitted default | Accepted values |
| --- | --- | --- |
| schemaVersion | Required | `itervyn.config/v1alpha1` |
| project.root | `.` | Existing directory, relative to config parent or absolute; no `..` components |
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
| providers.invaris.harness | `invaris.json` | Regular file directly in project root; basename only |
| providers.evaris.store | Required when enabled | Explicit absolute native store |
| providers.evaris.runId | Required when enabled | 32 lowercase hexadecimal characters; no latest selector |
| evidence.directory | `.itervyn/runs` | Relative path under root, without traversal, links or store/harness overlap |

At least one enabled provider must be required. Empty paths/executables, empty
explicit provider objects, fractional limits, integer overflow and duplicate
allowlist names are errors. Empty allowlist `[]` passes no additional variables.
A missing allowlisted environment variable stays absent. Evaris store/run fields
must be omitted when it is disabled; supplying empty strings is also invalid.
Missing native files/tools are runtime prerequisites and cannot become PASS.
Relative evidence paths must have their normalized spelling (no `./` prefix).

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

## Output

`--format json` emits one object; no progress text is mixed into stdout. Default
format is text. `version` reports version and supported schema lists. `init`
reports created/configPath/warnings; `doctor` emits
`itervyn.doctor/v1alpha1`. Verification/inspection use
[itervyn.result/v1alpha1](../schemas/result-v1alpha1.schema.json); preflight and
usage failures use [itervyn.diagnostic/v1alpha1](../schemas/diagnostic-v1alpha1.schema.json).

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
