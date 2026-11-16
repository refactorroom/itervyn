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
There is no filesystem/network isolation and no rollback of command side effects.

Explicit project/store anchors and config-parent anchors are canonicalized. Under
these anchors, links/special files and traversal are refused; reads/writes use
rooted filesystem operations. Relative evidence remains under the project root.
Exclusive creation prevents overwrites and collisions. This does not defend
against a malicious process running with the same user permissions replacing
trusted anchors, modifying tools or forging all evidence and hashes together.

Private directories use 0700 and files 0600. Original bounded provider streams and
config are preserved unchanged so stored hashes are meaningful. They may include
secrets, private paths or transcripts reported by a tool. Normal CLI output uses
safe application summaries rather than raw native logs. Redaction is not complete
and private evidence should not be uploaded publicly. Inspect and sanitize a copy
before sharing; retain the original hash/evidence relationship privately.

Git revision/dirty state is optional context, obtained through bounded read-only
commands with fsmonitor and optional locks disabled. It is not a working-tree
snapshot. Direct checks run against a live directory; concurrent edits are not
fully isolated. Harness and executable identities are rechecked, while the
trusted native tools remain responsible for their own observation semantics.

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
