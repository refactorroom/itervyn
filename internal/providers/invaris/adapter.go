// Package invaris adapts one supported native check profile, not guard logic.
package invaris

import (
	"context"
	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/process"
	"github.com/akkaraponph/itervyn/internal/verify"
	"os"
	"time"
)

type Adapter struct {
	Runner   process.Runner
	Identify func(string, string) (process.Identity, error)
}

func (Adapter) Name() string { return "invaris" }
func (a Adapter) Verify(ctx context.Context, q verify.Request) (out verify.Observation) {
	c := q.Config
	out.Result = verify.ProviderResult{Name: a.Name(), Status: verify.Error, NativeReasons: []string{}, Evidence: []verify.ArtifactRef{}, Scope: verify.Scope{Kind: "working-tree", ProjectRoot: verify.Ptr(c.ProjectRoot)}}
	fail := func(code string) verify.Observation { out.Result.Reason = verify.Ptr(code); return out }
	if ctx.Err() != nil {
		return fail(process.ContextReason(ctx))
	}
	if q.Strict() && (!q.SeparatedPolicy() || !verify.ExternalExecutable(c.ProjectRoot, c.Providers.Invaris.Executable)) {
		return fail("UNTRUSTED_POLICY")
	}
	b, err := c.ReadHarness()
	if err != nil {
		if os.IsNotExist(err) {
			return fail("HARNESS_MISSING")
		}
		return fail(config.Code(err))
	}
	h, err := ParseHarness(b)
	if err != nil {
		return fail(config.Code(err))
	}
	digest := config.Hash(b)
	path := c.HarnessPath
	if c.SchemaVersion == config.TrustedSchema {
		path = c.RuntimeHarnessPath
		out.Artifacts = append(out.Artifacts, verify.NativeArtifact{Path: "providers/invaris-harness.json", Data: b})
		staged, err := c.ReadRuntimeHarness()
		if err != nil || config.Hash(staged) != digest {
			return fail("EVIDENCE_MISMATCH")
		}
	}
	if q.Strict() {
		if h.StoreSensitiveOutput {
			return fail("UNTRUSTED_POLICY")
		}
		for _, g := range h.Guards {
			if !verify.ExternalExecutable(c.ProjectRoot, g.Program) || len(g.EnvAllowlist) != 0 {
				return fail("UNTRUSTED_POLICY")
			}
		}
	}
	identify := a.Identify
	if identify == nil {
		identify = process.Identify
	}
	id, err := identify(c.Providers.Invaris.Executable, a.Name())
	if err != nil {
		return fail(config.Code(err))
	}
	tmp, err := os.MkdirTemp("", "itervyn-process-")
	if err != nil {
		return fail("IO_ERROR")
	}
	defer os.RemoveAll(tmp)
	env := process.Environment(c.Execution.EnvironmentAllowlist, tmp)
	if q.Strict() {
		env = []string{"PATH=/usr/bin:/bin", "TMPDIR=" + tmp}
	}
	s := process.Spec{Executable: id.Path, ExpectedSHA256: id.SHA256, Args: []string{"check", "--format", "json", path}, Dir: c.ProjectRoot, Env: env, Timeout: time.Duration(c.Execution.ProviderTimeoutSeconds) * time.Second, StdoutLimit: c.Execution.StdoutLimitBytes, StderrLimit: c.Execution.StderrLimitBytes}
	runner := a.Runner
	if runner == nil {
		runner = process.Local{}
	}
	o := runner.Run(ctx, s)
	out.Result.ExitCode = o.ExitCode
	out.Result.StartedAt = verify.Ptr(o.StartedAt)
	out.Result.FinishedAt = verify.Ptr(o.FinishedAt)
	out.Result.ExecutableSHA256 = verify.Ptr(id.SHA256)
	out.Artifacts = append([]verify.NativeArtifact{
		{Path: "providers/invaris.json", Data: o.Stdout},
		{Path: "providers/invaris.stderr", Data: o.Stderr},
		{Path: "providers/invaris.process.json", Data: process.RecordJSON(id, s, o)},
	}, out.Artifacts...)
	if o.Reason != "" {
		return fail(o.Reason)
	}
	if o.ExecutableSHA256 != id.SHA256 {
		return fail("EVIDENCE_MISMATCH")
	}
	after, err := c.ReadRuntimeHarness()
	if err != nil || config.Hash(after) != digest {
		return fail("EVIDENCE_MISMATCH")
	}
	if c.SchemaVersion == config.TrustedSchema {
		source, err := c.ReadHarness()
		if err != nil || config.Hash(source) != digest {
			return fail("EVIDENCE_MISMATCH")
		}
	}
	r, err := validateReport(o.Stdout, h, c.ProjectRoot, digest, o.ExitCode)
	if err != nil {
		return fail(config.Code(err))
	}
	out.Git = &verify.GitSnapshot{Revision: r.GitRevision, Dirty: r.GitDirty}
	out.Incomplete = r.incompleteEvidence()
	out.Result.NativeDecision = verify.Ptr(r.Decision)
	out.Result.NativeSchemaVersion = verify.Ptr(r.SchemaVersion)
	switch r.Decision {
	case "ACCEPT":
		out.Result.Status = verify.Pass
	case "REJECT":
		out.Result.Status = verify.Fail
	case "REFUSE":
		out.Result.Status = verify.Inconclusive
	}
	if q.Strict() && len(r.Compatibility.Warnings) != 0 {
		out.Result.NativeReasons = append(out.Result.NativeReasons, "COMPATIBILITY_WARNING")
	}
	if q.Strict() {
		for _, o := range r.Observations {
			for _, d := range o.Diagnostics {
				out.Result.NativeReasons = append(out.Result.NativeReasons, d.Code)
			}
		}
	}
	for _, d := range r.Diagnostics {
		out.Result.NativeReasons = append(out.Result.NativeReasons, d.Code)
	}
	for _, c := range r.Checks {
		for _, d := range c.Diagnostics {
			out.Result.NativeReasons = append(out.Result.NativeReasons, d.Code)
		}
	}
	return out
}
