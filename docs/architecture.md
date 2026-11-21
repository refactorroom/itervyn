# Architecture

Itervyn coordinates one local verification operation. The agent owns the correction
loop. CLI parsing depends on the verification application; adapters implement its
Provider interface and use explicit process boundaries. The application does not
import adapters. Evidence storage implements the application-owned Store interface.
No public Go API, registry, scheduler, server, guard engine or grading engine exists.

The original code uses Apache-2.0. External Invaris and Evaris binaries are separate
requirements. Native report DTOs follow the Apache-2.0 schema contracts of the supported
release lines;
no implementation or skills are vendored. `testdata` contains authored controls,
not real-agent performance measurements.

## One verification operation

1. Strictly decode and resolve configuration without starting commands.
2. Capture Git provenance, reserve an exclusive random run directory,
   and preserve exact config bytes. The revision gate requires starting
   provenance and an evidence directory outside the candidate.
3. Execute enabled Invaris then Evaris within the shared deadline. Each adapter
   validates its concrete native contract and binds evidence to the requested
   subject. Disabled providers do not resolve executables.
4. Persist original bounded output and execution identity. The revision gate
   checks ending Git provenance, native Invaris provenance and required
   evidence reliability. Aggregate the decisions and publish the checksum
   marker last.
5. Render only after publication. Inspection checks committed local evidence
   without re-execution or mutation.

Known required FAIL dominates required unavailable/inconclusive results. Otherwise
any required non-PASS is INCONCLUSIVE; all required PASS gives PASS. Optional
results remain warnings. An Itervyn orchestration/publication failure gives ERROR,
retaining any trustworthy provider failure. No enabled required provider means
invalid configuration, never vacuous success. Recommendations are advisory.

Configuration and the revision gate own selection and requiredness. Adapters report observations. Native
Invaris harnesses own guard definitions. Frozen Evaris inputs own evaluation
criteria. Live project checks and imported-run evaluation remain separate scopes.
The narrow native contracts are deliberate integration boundaries, not a generic
plugin system or a second guard/grading implementation.
