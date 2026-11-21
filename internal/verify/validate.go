package verify

import (
	"github.com/akkaraponph/itervyn/internal/config"
	"path/filepath"
	"reflect"
)

func KnownCode(code string) bool {
	switch code {
	case "INVALID_ARGUMENT", "INVALID_CONFIG", "UNSUPPORTED_SCHEMA", "UNSUPPORTED_TOOL", "UNSUPPORTED_ACTION", "PATH_UNSAFE", "TOOL_MISSING", "HARNESS_MISSING", "PREREQUISITE_MISSING", "TIMEOUT", "CANCELED", "OUTPUT_LIMIT", "OUTPUT_MISSING", "OUTPUT_INVALID", "EXIT_MISMATCH", "EVIDENCE_MISMATCH", "EVIDENCE_CORRUPT", "IO_ERROR", "INTERNAL_ERROR", "INCOMPLETE_RUN", "RUN_NOT_FOUND",
		"UNTRUSTED_POLICY", "REVISION_REQUIRED", "GIT_UNAVAILABLE", "GIT_STATUS_UNAVAILABLE", "REVISION_MISMATCH", "REVISION_MOVED", "GIT_DIRTY", "PROVENANCE_CONFLICT", "LIVE_EVIDENCE_REQUIRED", "EVIDENCE_INCOMPLETE":
		return true
	}
	return false
}
func ValidStatus(s Status) bool { return s == Pass || s == Fail || s == Inconclusive || s == Error }

// ValidateResult validates owned metadata independently of the aggregate status.
// It does not re-run native checks or authenticate evidence against a signer.
func ValidateResult(r Result) error {
	bad := func() error { return config.Err("EVIDENCE_CORRUPT", "result invariants") }
	if r.SchemaVersion != ResultSchema && r.SchemaVersion != GatedResultSchema {
		return config.Err("UNSUPPORTED_SCHEMA", "result version")
	}
	if (r.SchemaVersion == GatedResultSchema) != (r.RevisionGate != nil) {
		return bad()
	}
	if !config.IDPattern.MatchString(r.RunID) || r.TaskID != nil || !ValidStatus(r.Status) || r.StartedAt.IsZero() || r.FinishedAt.IsZero() || r.FinishedAt.Before(r.StartedAt) || r.ConfigSHA256 == nil || !config.DigestPattern.MatchString(*r.ConfigSHA256) || r.ProjectRoot == nil || !filepath.IsAbs(*r.ProjectRoot) || !r.EvidencePersisted || r.Providers == nil || r.Feedback == nil || r.Warnings == nil || r.Artifacts == nil {
		return bad()
	}
	if !validRevision(r.GitRevision) {
		return bad()
	}
	for _, f := range append(append([]Feedback{}, r.Feedback...), r.Warnings...) {
		if !KnownCode(f.Code) && f.Code != "NATIVE_OUTCOME" {
			return bad()
		}
		if f.Provider != nil && *f.Provider != "invaris" && *f.Provider != "evaris" {
			return bad()
		}
		if f.EvidencePath != nil && !config.SafeRelative(*f.EvidencePath) {
			return bad()
		}
	}
	seen := map[string]bool{}
	for _, p := range r.Providers {
		if (p.Name != "invaris" && p.Name != "evaris") || seen[p.Name] || !ValidStatus(p.Status) || p.NativeReasons == nil || p.Evidence == nil {
			return bad()
		}
		seen[p.Name] = true
		if p.Reason != nil && !KnownCode(*p.Reason) {
			return bad()
		}
		if p.Status == Error && p.Reason == nil {
			return bad()
		}
		if p.ExecutableSHA256 != nil && !config.DigestPattern.MatchString(*p.ExecutableSHA256) {
			return bad()
		}
		if (p.StartedAt == nil) != (p.FinishedAt == nil) || p.StartedAt != nil && (p.StartedAt.IsZero() || p.FinishedAt.Before(*p.StartedAt)) {
			return bad()
		}
		s := p.Scope
		if p.Name == "invaris" {
			if s.Kind != "working-tree" || s.ProjectRoot == nil || *s.ProjectRoot != *r.ProjectRoot || s.Store != nil || s.RunID != nil {
				return bad()
			}
		} else {
			if s.Kind != "imported-run" || s.Store == nil || !filepath.IsAbs(*s.Store) || s.RunID == nil || !config.IDPattern.MatchString(*s.RunID) || s.ProjectRoot != nil {
				return bad()
			}
		}
		for _, id := range []*string{s.RunID, s.PreparationID, s.EvaluationID} {
			if id != nil && !config.IDPattern.MatchString(*id) {
				return bad()
			}
		}
		for _, d := range []*string{s.RunDigest, s.OracleDigest} {
			if d != nil && !config.DigestPattern.MatchString(*d) {
				return bad()
			}
		}
		for _, id := range []*string{s.ExperimentID, s.CaseID, s.ArmID} {
			if id != nil && *id == "" {
				return bad()
			}
		}
		if p.Status != Error {
			if p.NativeDecision == nil || p.NativeSchemaVersion == nil || p.ExitCode == nil || p.ExecutableSHA256 == nil || p.StartedAt == nil || len(p.Evidence) == 0 || p.Reason != nil {
				return bad()
			}
			wantNative, wantSchema, wantExit := string(p.Status), "evaris.evaluation/v1alpha1", Exit(p.Status)
			if p.Name == "invaris" {
				wantSchema = "invaris.evidence/v1alpha2"
				switch p.Status {
				case Pass:
					wantNative = "ACCEPT"
				case Fail:
					wantNative = "REJECT"
				case Inconclusive:
					wantNative = "REFUSE"
				}
			}
			if *p.NativeDecision != wantNative || *p.NativeSchemaVersion != wantSchema || *p.ExitCode != wantExit {
				return bad()
			}
			if p.Name == "evaris" && (s.ExperimentID == nil || s.PreparationID == nil || s.EvaluationID == nil || s.CaseID == nil || s.ArmID == nil || s.RunDigest == nil || s.OracleDigest == nil) {
				return bad()
			}
		}
	}
	s, n := Aggregate(r.Providers)
	if g := r.RevisionGate; g != nil {
		if !validRevision(g.ExpectedRevision) || !validSnapshot(g.Before) || !validSnapshot(g.After) ||
			!reflect.DeepEqual(g.Before, GitSnapshot{Revision: r.GitRevision, Dirty: r.GitDirty}) ||
			g.Incomplete == nil || (g.Invaris != nil && (!seen["invaris"] || !validSnapshot(*g.Invaris))) {
			return bad()
		}
		recorded := map[string]bool{}
		for _, name := range g.Incomplete {
			if !seen[name] || recorded[name] {
				return bad()
			}
			recorded[name] = true
		}
		if len(g.blockers(r.Providers)) > 0 && s == Pass {
			s, n = Inconclusive, Investigate
		}
	}
	if r.Status == Inconclusive && r.NextAction == Stop {
		for _, f := range r.Feedback {
			if f.Code == "CANCELED" {
				n = Stop
			}
		}
	}
	if r.Status == Error {
		if r.NextAction != Stop {
			return bad()
		}
	} else if r.Status != s || r.NextAction != n {
		return bad()
	}
	return nil
}

func validRevision(revision *string) bool {
	return revision == nil || config.RevisionPattern.MatchString(*revision)
}

func validSnapshot(g GitSnapshot) bool {
	return validRevision(g.Revision)
}
