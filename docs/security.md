# Security model

Itervyn executes trusted local programs. It is not a sandbox. Review configuration,
native harness commands, provider executables and explicit Evaris stores before
use. Repository/agent text and provider findings never become shell instructions
or arbitrary filesystem read paths. Version metadata and hashes identify builds
and bytes; they cannot prove tool honesty, authorship or model quality.

The process boundary uses explicit executable/argv, Unix process groups, context
deadlines, bounded stdout/stderr and TERM followed by KILL. Capturing valid PASS
JSON does not override a timeout, output limit, signal, exit mismatch or invalid
evidence. Captures default to 4 MiB stdout and 64 KiB stderr, separately. Limit
crossing cancels the group and retains only the capped prefix.

Only PATH, an owned TMPDIR and explicitly allowlisted existing environment
variables are passed. Environment values are not logged. Trusted harness tools
may invoke their own shells, interpreters, network or subprocesses. A detached
descendant that escapes its process group is outside termination guarantees.
These local process controls provide no filesystem or network isolation and no rollback of command side effects.

Explicit project/store anchors and config-parent anchors are canonicalized. Under
these anchors, links/special files and traversal are refused; reads/writes use
rooted filesystem operations. Relative evidence remains under the project root. The revision gate requires an external destination.
Exclusive creation prevents overwrites and collisions. This does not defend
against a malicious process running with the same user permissions replacing
trusted anchors, modifying tools or forging all evidence and hashes together.

Private directories use 0700 and files 0600. Original bounded provider streams and
config are preserved unchanged so stored hashes are meaningful. They may include
secrets, private paths or transcripts reported by a tool. Normal CLI output uses
safe application summaries rather than raw native logs. Redaction is not complete
and private evidence should not be uploaded publicly. Inspect and sanitize a copy
before sharing; retain the original hash/evidence relationship privately.

Ungated Git provenance is optional. The revision gate samples Git before and
after provider execution and compares native Invaris provenance. These are
point-in-time observations. They do not detect every change-and-revert between
observations or prove continuous immutability. Git status also has no knowledge
of empty directories, external files or the contents of ignored paths after
they have been removed. CI begins with a new checkout without materialized
submodules and exposes it read-only to the verifying processes.

The [CI launcher](integration.md#ci-enforcement) uses a credential-free,
network-disabled container. Candidate files are data read by trusted programs.
The container writes run evidence only to a separate mount. Treat background
processes on the host, the container runtime, the mounted policy tools and
the launcher as trusted. Itervyn checks native harness digest equality even if
a file changed and reverted during the invocation. The CI mount additionally
prevents writes to the staged harness, policy or candidate by container
processes. A direct CLI invocation on a writable filesystem provides no
equivalent protection. A private temp directory does not isolate programs
sharing its owner's privileges.

If a future guard must run candidate code, isolate that execution from the
trusted verifier, native provider and evidence writer with a separate OS or
container boundary. Pass only scoped observations across it. The current
verifier container's evidence mount is writable by its own processes.

## Interruption and recovery

COMMITTED is published last and contains the result checksum. A run without a
marker is incomplete, not PASS. inspect validates its own result and referenced
artifact hashes without launching providers; it does not audit the entire external
Evaris store or need that store to remain mounted.

SIGINT/SIGTERM attempt bounded process-group cleanup and a canceled record. The
shared execution deadline has up to 2 seconds of process cleanup and a separate
2-second finalization context. Kernel filesystem calls such as fsync cannot be
forcibly interrupted by a Go context; finalization deadlines are checked between
operations. SIGKILL, power loss, disk failure or invalid configuration before safe
storage can prevent durable publication. Preflight failures return an explicit
diagnostic with evidencePersisted false. Do not repair a marker or relabel an
incomplete run. Retain failed-operation records for diagnosis and start a new run.

Completed Itervyn/Evaris records are append-only. Rolling back a binary does not
delete native evaluations already created. There is no automatic resume, retry,
garbage collection, store migration or cleanup of completed evidence.
