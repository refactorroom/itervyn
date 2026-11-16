// Package invaris adapts one pinned native check profile, not guard logic.
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
	b, err := config.Read(c.ProjectRoot, c.Providers.Invaris.Harness, config.MaxConfig)
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
	s := process.Spec{Executable: id.Path, ExpectedSHA256: id.SHA256, Args: []string{"check", "--format", "json", c.HarnessPath}, Dir: c.ProjectRoot, Env: process.Environment(c.Execution.EnvironmentAllowlist, tmp), Timeout: time.Duration(c.Execution.ProviderTimeoutSeconds) * time.Second, StdoutLimit: c.Execution.StdoutLimitBytes, StderrLimit: c.Execution.StderrLimitBytes}
	runner := a.Runner
	if runner == nil {
		runner = process.Local{}
	}
	o := runner.Run(ctx, s)
	out.Result.ExitCode = o.ExitCode
	out.Result.StartedAt = verify.Ptr(o.StartedAt)
	out.Result.FinishedAt = verify.Ptr(o.FinishedAt)
	out.Result.ExecutableSHA256 = verify.Ptr(id.SHA256)
	out.Artifacts = []verify.NativeArtifact{{Path: "providers/invaris.json", Data: o.Stdout}, {Path: "providers/invaris.stderr", Data: o.Stderr}, {Path: "providers/invaris.process.json", Data: process.RecordJSON(id, s, o)}}
	if o.Reason != "" {
		return fail(o.Reason)
	}
	if o.ExecutableSHA256 != id.SHA256 {
		return fail("EVIDENCE_MISMATCH")
	}
	after, err := config.Read(c.ProjectRoot, c.Providers.Invaris.Harness, config.MaxConfig)
	if err != nil || config.Hash(after) != config.Hash(b) {
		return fail("EVIDENCE_MISMATCH")
	}
	r, err := validateReport(o.Stdout, h, c.ProjectRoot, config.Hash(b), o.ExitCode)
	if err != nil {
		return fail(config.Code(err))
	}
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
