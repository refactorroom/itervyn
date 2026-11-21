package verify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/process"
)

func newID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func (s Service) Verify(parent context.Context, q Request) (r Result, err error) {
	started := q.StartedAt
	if started.IsZero() {
		started = time.Now().UTC()
	}
	r = Result{SchemaVersion: ResultSchema, RunID: q.RunID, Status: Error, NextAction: Stop, Providers: []ProviderResult{}, Feedback: []Feedback{}, Warnings: []Feedback{}, StartedAt: started, ConfigSHA256: Ptr(q.Config.ConfigSHA256), ProjectRoot: Ptr(q.Config.ProjectRoot), Artifacts: []ArtifactRef{}}
	opError := func(code string, e error) (Result, error) {
		r.Status, r.NextAction, r.EvidencePersisted = Error, Stop, false
		r.FinishedAt = time.Now().UTC()
		r.Feedback = append(r.Feedback, Feedback{Code: code, Message: "Verification operation could not be completed reliably."})
		return r, e
	}
	if s.Store == nil {
		return opError("INTERNAL_ERROR", config.Err("INTERNAL_ERROR", "missing store"))
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(q.Config.Execution.TimeoutSeconds)*time.Second)
	defer cancel()
	if q.Strict() {
		if q.ExpectedRevision != "" && !config.RevisionPattern.MatchString(q.ExpectedRevision) {
			return opError("INVALID_ARGUMENT", config.Err("INVALID_ARGUMENT", "expected revision must be a full lowercase commit ID"))
		}
		r.SchemaVersion = GatedResultSchema
		gate := &RevisionGate{SeparatedPolicy: q.SeparatedPolicy(), Before: strictGit(ctx, q), Incomplete: []string{}}
		if q.ExpectedRevision != "" {
			gate.ExpectedRevision = Ptr(q.ExpectedRevision)
		}
		r.RevisionGate = gate
		r.GitRevision, r.GitDirty = gate.Before.Revision, gate.Before.Dirty
		if !filepath.IsAbs(q.Config.EvidenceDir) || config.Within(q.Config.ProjectRoot, q.Config.EvidenceDir) {
			return opError("UNTRUSTED_POLICY", config.Err("UNTRUSTED_POLICY", "revision gate evidence must be outside the candidate root"))
		}
	} else {
		r.GitRevision, r.GitDirty = gitMetadata(ctx, q.Config.ProjectRoot)
		if r.GitRevision == nil {
			r.Warnings = append(r.Warnings, Feedback{Code: "PREREQUISITE_MISSING", Message: "Git provenance is unavailable."})
		}
	}
	var h RunHandle
	for attempt := 0; attempt < 3; attempt++ {
		if q.RunID == "" || attempt > 0 {
			q.RunID, err = newID()
			if err != nil {
				return opError("IO_ERROR", err)
			}
		}
		r.RunID = q.RunID
		h, err = s.Store.Begin(ctx, q.Config, q.RunID)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return opError(config.Code(err), err)
		}
	}
	if err != nil {
		return opError("IO_ERROR", err)
	}
	q.StartedAt = started
	put := func(a NativeArtifact) (ArtifactRef, error) {
		ref, e := s.Store.Put(ctx, h, a)
		if e == nil {
			r.Artifacts = append(r.Artifacts, ref)
		}
		return ref, e
	}
	if _, err = put(NativeArtifact{Path: "config.json", Data: q.Config.ConfigBytes}); err != nil {
		return opError(config.Code(err), err)
	}
	providers := map[string]Provider{}
	for _, p := range s.Providers {
		if p == nil {
			return opError("INTERNAL_ERROR", config.Err("INTERNAL_ERROR", "nil provider"))
		}
		name := p.Name()
		if providers[name] != nil || (name != "invaris" && name != "evaris") {
			return opError("INTERNAL_ERROR", config.Err("INTERNAL_ERROR", "adapter registry mismatch"))
		}
		providers[name] = p
	}
	processRecords := []json.RawMessage{}
	var finalize context.Context

	for _, p := range []struct {
		name              string
		enabled, required bool
	}{{"invaris", q.Config.Providers.Invaris.Enabled, q.Config.Providers.Invaris.Required}, {"evaris", q.Config.Providers.Evaris.Enabled, q.Config.Providers.Evaris.Required}} {
		if !p.enabled {
			continue
		}
		adapter := providers[p.name]
		if adapter == nil {
			return opError("INTERNAL_ERROR", config.Err("INTERNAL_ERROR", "enabled adapter missing"))
		}
		out := adapter.Verify(ctx, q)
		if out.Result.Name != p.name {
			return opError("INTERNAL_ERROR", config.Err("INTERNAL_ERROR", "provider identity mismatch"))
		}
		out.Result.Required = p.required || q.Strict() && p.name == "invaris"
		if g := r.RevisionGate; g != nil {
			if p.name == "invaris" {
				g.Invaris = out.Git
			}
			if out.Incomplete && (p.required || p.name == "invaris") {
				g.Incomplete = append(g.Incomplete, p.name)
			}
		}
		out.Result.Evidence = []ArtifactRef{}
		if out.Result.NativeReasons == nil {
			out.Result.NativeReasons = []string{}
		}
		for _, a := range out.Artifacts {
			// A canceled provider still gets its retained observation committed.
			persistCtx := ctx
			if ctx.Err() != nil {
				if finalize == nil {
					finalCtx, stopFinal := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
					defer stopFinal()
					finalize = finalCtx
				}
				persistCtx = finalize
			}
			ref, e := s.Store.Put(persistCtx, h, a)
			if e != nil {
				return opError(config.Code(e), e)
			}
			r.Artifacts = append(r.Artifacts, ref)
			out.Result.Evidence = append(out.Result.Evidence, ref)
			if strings.HasSuffix(a.Path, ".process.json") {
				processRecords = append(processRecords, json.RawMessage(a.Data))
			}
		}
		r.Providers = append(r.Providers, out.Result)
		if out.Result.Status != Pass || len(out.Result.NativeReasons) > 0 {
			f := Feedback{Provider: Ptr(p.name), Code: "NATIVE_OUTCOME", Message: "Inspect private provider evidence and recorded scope."}
			if out.Result.Reason != nil {
				f.Code = *out.Result.Reason
			}
			if len(out.Result.Evidence) > 0 {
				f.EvidencePath = Ptr(out.Result.Evidence[0].Path)
			}
			if out.Result.Required && out.Result.Status != Pass {
				r.Feedback = append(r.Feedback, f)
			} else {
				r.Warnings = append(r.Warnings, f)
			}
		}
	}
	r.Status, r.NextAction = Aggregate(r.Providers)
	if g := r.RevisionGate; g != nil {
		g.After = strictGit(ctx, q)
		if blockers := g.blockers(r.Providers); len(blockers) > 0 {
			r.Feedback = append(r.Feedback, blockers...)
			if r.Status == Pass {
				r.Status, r.NextAction = Inconclusive, Investigate
			}
		}
	}
	if ctx.Err() != nil && r.Status == Pass {
		r.Status, r.NextAction = Error, Stop
		r.Feedback = append(r.Feedback, Feedback{Code: process.ContextReason(ctx), Message: "Verification was interrupted before publication."})
	}
	if errors.Is(ctx.Err(), context.Canceled) && r.Status == Inconclusive {
		r.NextAction = Stop
	}
	// Finalization has one shared two-second budget, independent of cancellation.
	if finalize == nil {
		finalCtx, stopFinal := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer stopFinal()
		finalize = finalCtx
	}
	finalCtx := finalize
	b, e := json.MarshalIndent(processRecords, "", "  ")
	if e != nil {
		return opError("INTERNAL_ERROR", e)
	}
	ref, e := s.Store.Put(finalCtx, h, NativeArtifact{Path: "processes.json", Data: append(b, '\n')})
	if e != nil {
		return opError("IO_ERROR", e)
	}
	r.Artifacts = append(r.Artifacts, ref)
	if ctx.Err() != nil && r.Status == Pass {
		r.Status, r.NextAction = Error, Stop
		r.Feedback = append(r.Feedback, Feedback{Code: process.ContextReason(ctx), Message: "Verification was interrupted before publication."})
	}
	r.FinishedAt = time.Now().UTC()
	r.EvidencePersisted = true
	if errors.Is(ctx.Err(), context.Canceled) && r.Status == Inconclusive {
		r.NextAction = Stop
		r.Feedback = append(r.Feedback, Feedback{Code: "CANCELED", Message: "Verification was canceled before publication."})
	}
	commitCtx := finalCtx
	if r.Status == Pass {
		commitCtx = ctx
	}
	if err = s.Store.Commit(commitCtx, h, r); err != nil {
		code := "IO_ERROR"
		if ctx.Err() != nil {
			code = process.ContextReason(ctx)
		}
		return opError(code, err)
	}
	return r, nil
}
func gitMetadata(ctx context.Context, dir string) (*string, *bool) {
	args := []string{"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false"}
	s := process.Spec{Executable: "git", Args: append(append([]string{}, args...), "rev-parse", "HEAD"), Dir: dir, Env: []string{"PATH=" + os.Getenv("PATH"), "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}, Timeout: time.Second, StdoutLimit: 65536, StderrLimit: 65536}
	a := (process.Local{}).Run(ctx, s)
	if a.Reason != "" || a.ExitCode == nil || *a.ExitCode != 0 {
		return nil, nil
	}
	rev := strings.TrimSpace(string(a.Stdout))
	if len(rev) != 40 && len(rev) != 64 {
		return nil, nil
	}
	s.Args = append(append([]string{}, args...), "status", "--porcelain", "--untracked-files=normal")
	b := (process.Local{}).Run(ctx, s)
	if b.Reason != "" || b.ExitCode == nil || *b.ExitCode != 0 {
		return Ptr(rev), nil
	}
	return Ptr(rev), Ptr(len(b.Stdout) > 0)
}
