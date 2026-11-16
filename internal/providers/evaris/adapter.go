// Package evaris evaluates an explicit imported run through the trusted native CLI.
package evaris

import (
	"bytes"
	"context"
	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/process"
	"github.com/akkaraponph/itervyn/internal/verify"
	"os"
	"path/filepath"
	"time"
)

type Adapter struct {
	Runner   process.Runner
	Identify func(string, string) (process.Identity, error)
}

func (Adapter) Name() string { return "evaris" }
func Prerequisites(c config.Resolved) error {
	for _, p := range []string{"COMMITTED", filepath.Join("runs", c.Providers.Evaris.RunID, "COMMITTED")} {
		if _, err := config.Read(c.EvarisStore, p, 1024); err != nil {
			return config.Err("PREREQUISITE_MISSING", "committed Evaris store/import required")
		}
	}
	return nil
}
func (a Adapter) Verify(ctx context.Context, q verify.Request) (out verify.Observation) {
	c := q.Config
	v := c.Providers.Evaris
	out.Result = verify.ProviderResult{Name: a.Name(), Status: verify.Error, NativeReasons: []string{}, Evidence: []verify.ArtifactRef{}, Scope: verify.Scope{Kind: "imported-run", Store: verify.Ptr(c.EvarisStore), RunID: verify.Ptr(v.RunID)}}
	fail := func(code string) verify.Observation { out.Result.Reason = verify.Ptr(code); return out }
	if ctx.Err() != nil {
		return fail(process.ContextReason(ctx))
	}
	if err := Prerequisites(c); err != nil {
		return fail(config.Code(err))
	}
	identify := a.Identify
	if identify == nil {
		identify = process.Identify
	}
	id, err := identify(v.Executable, a.Name())
	if err != nil {
		return fail(config.Code(err))
	}
	tmp, err := os.MkdirTemp("", "itervyn-process-")
	if err != nil {
		return fail("IO_ERROR")
	}
	defer os.RemoveAll(tmp)
	s := process.Spec{Executable: id.Path, ExpectedSHA256: id.SHA256, Args: []string{"evaluate", "--store", c.EvarisStore, "--run", v.RunID}, Dir: c.ProjectRoot, Env: process.Environment(c.Execution.EnvironmentAllowlist, tmp), Timeout: time.Duration(c.Execution.ProviderTimeoutSeconds) * time.Second, StdoutLimit: c.Execution.StdoutLimitBytes, StderrLimit: c.Execution.StderrLimitBytes}
	runner := a.Runner
	if runner == nil {
		runner = process.Local{}
	}
	o := runner.Run(ctx, s)
	out.Result.ExitCode = o.ExitCode
	out.Result.StartedAt = verify.Ptr(o.StartedAt)
	out.Result.FinishedAt = verify.Ptr(o.FinishedAt)
	out.Result.ExecutableSHA256 = verify.Ptr(id.SHA256)
	out.Artifacts = []verify.NativeArtifact{{Path: "providers/evaris.json", Data: o.Stdout}, {Path: "providers/evaris.stderr", Data: o.Stderr}, {Path: "providers/evaris.process.json", Data: process.RecordJSON(id, s, o)}}
	if o.Reason != "" {
		return fail(o.Reason)
	}
	if o.ExecutableSHA256 != id.SHA256 {
		return fail("EVIDENCE_MISMATCH")
	}
	e, err := parse(o.Stdout, v.RunID, o.ExitCode)
	if err != nil {
		return fail(config.Code(err))
	}
	rel := filepath.Join("evaluations", e.ID)
	b, err := config.Read(c.EvarisStore, filepath.Join(rel, "evaluation.json"), config.MaxConfig)
	if err != nil || !bytes.Equal(b, o.Stdout) {
		return fail("EVIDENCE_MISMATCH")
	}
	if _, err = config.Read(c.EvarisStore, filepath.Join(rel, "COMMITTED"), 1024); err != nil {
		return fail("EVIDENCE_MISMATCH")
	}
	out.Artifacts = append(out.Artifacts, verify.NativeArtifact{Path: "providers/evaris-evaluation.json", Data: b})
	out.Result.Status = verify.Status(e.Assessment)
	out.Result.NativeDecision = verify.Ptr(e.Assessment)
	out.Result.NativeSchemaVersion = verify.Ptr(e.SchemaVersion)
	out.Result.Scope.PreparationID = verify.Ptr(e.PreparationID)
	out.Result.Scope.EvaluationID = verify.Ptr(e.ID)
	out.Result.Scope.ExperimentID = verify.Ptr(e.ExperimentID)
	out.Result.Scope.CaseID = verify.Ptr(e.CaseID)
	out.Result.Scope.ArmID = verify.Ptr(e.ArmID)
	out.Result.Scope.RunDigest = verify.Ptr(e.RunDigest)
	out.Result.Scope.OracleDigest = verify.Ptr(e.OracleDigest)
	for _, g := range e.Graders {
		if g.Reason != "" {
			out.Result.NativeReasons = append(out.Result.NativeReasons, g.Reason)
		}
	}
	if e.Verification != nil && e.Verification.Reason != "" {
		out.Result.NativeReasons = append(out.Result.NativeReasons, e.Verification.Reason)
	}
	return out
}
