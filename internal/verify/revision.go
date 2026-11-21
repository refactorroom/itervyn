package verify

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/process"
)

type GitSnapshot struct {
	Revision *string `json:"revision"`
	Dirty    *bool   `json:"dirty"`
}

type RevisionGate struct {
	ExpectedRevision *string      `json:"expectedRevision"`
	SeparatedPolicy  bool         `json:"separatedPolicy"`
	Before           GitSnapshot  `json:"before"`
	After            GitSnapshot  `json:"after"`
	Invaris          *GitSnapshot `json:"invaris"`
	Incomplete       []string     `json:"incompleteProviders"`
}

func (q Request) Strict() bool {
	return q.ExpectedRevision != "" || q.Config.Project.RequireCleanTree
}

func (q Request) SeparatedPolicy() bool {
	c := q.Config
	if c.SchemaVersion != config.TrustedSchema || !filepath.IsAbs(c.ConfigPath) || config.Within(c.ProjectRoot, c.ConfigPath) ||
		config.Within(c.ProjectRoot, c.HarnessPath) || !filepath.IsAbs(c.EvidenceDir) ||
		config.Within(c.ProjectRoot, c.EvidenceDir) || len(c.Execution.EnvironmentAllowlist) != 0 {
		return false
	}
	return true
}

// ExternalExecutable requires an explicit path whose resolved location is
// outside the subject. The launcher is responsible for its write protection.
func ExternalExecutable(root, path string) bool {
	if !filepath.IsAbs(path) || !config.SafeLexical(path) || config.Within(root, path) {
		return false
	}
	actual, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil || config.Within(root, filepath.Join(actual, filepath.Base(path))) {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

func (g RevisionGate) blockers(providers []ProviderResult) []Feedback {
	var findings []Feedback
	add := func(code, message string) {
		findings = append(findings, Feedback{Code: code, Message: message})
	}
	if !g.SeparatedPolicy {
		add("UNTRUSTED_POLICY", "The revision gate needs external trusted configuration, harness and evidence paths with an empty environment allowlist.")
	}
	switch {
	case g.ExpectedRevision == nil:
		add("REVISION_REQUIRED", "Supply the full expected candidate commit ID.")
	case g.Before.Revision == nil || g.After.Revision == nil:
		add("GIT_UNAVAILABLE", "Git provenance is unavailable at a verification boundary.")
	case *g.Before.Revision != *g.After.Revision:
		add("REVISION_MOVED", "Candidate HEAD changed between Git observations.")
	case *g.Before.Revision != *g.ExpectedRevision:
		add("REVISION_MISMATCH", "Observed HEAD differs from the expected candidate commit.")
	}
	if g.Before.Dirty == nil || g.After.Dirty == nil {
		add("GIT_STATUS_UNAVAILABLE", "Candidate worktree status could not be established.")
	} else if *g.Before.Dirty || *g.After.Dirty {
		add("GIT_DIRTY", "Tracked, untracked or ignored candidate inputs differ from the controlled checkout.")
	}
	livePass := false
	for _, p := range providers {
		if p.Name == "invaris" && p.Status == Pass {
			livePass = true
		}
	}
	if !livePass {
		add("LIVE_EVIDENCE_REQUIRED", "A passing live Invaris check is required for candidate verification.")
	}
	if g.Invaris == nil && livePass || g.Invaris != nil && !g.matchesInvaris() {
		add("PROVENANCE_CONFLICT", "Live Invaris evidence does not report the expected clean candidate commit.")
	}
	for _, name := range g.Incomplete {
		f := Feedback{Provider: Ptr(name), Code: "EVIDENCE_INCOMPLETE", Message: "Required provider evidence has a reliability diagnostic."}
		findings = append(findings, f)
	}
	return findings
}

func (g RevisionGate) matchesInvaris() bool {
	native := g.Invaris
	if native.Revision == nil || native.Dirty == nil || *native.Dirty {
		return false
	}
	for _, observed := range []*string{g.ExpectedRevision, g.Before.Revision, g.After.Revision} {
		if observed != nil && *observed != *native.Revision {
			return false
		}
	}
	return true
}

func strictGit(ctx context.Context, q Request) GitSnapshot {
	dir := q.Config.ProjectRoot
	run := func(args ...string) ([]byte, bool) {
		fixed := []string{"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.ignoreStat=false", "-c", "core.hooksPath=/dev/null"}
		s := process.Spec{Executable: "/usr/bin/git", Args: append(fixed, args...), Dir: dir,
			Env:     []string{"PATH=/usr/bin:/bin", "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"},
			Timeout: 5 * time.Second, StdoutLimit: 4 << 20, StderrLimit: 65536}
		o := (process.Local{}).Run(ctx, s)
		return o.Stdout, o.Reason == "" && o.ExitCode != nil && *o.ExitCode == 0
	}
	top, ok := run("rev-parse", "--show-toplevel")
	if !ok || strings.TrimSuffix(string(top), "\n") != dir {
		return GitSnapshot{}
	}
	raw, ok := run("rev-parse", "--verify", "HEAD^{commit}")
	rev := strings.TrimSpace(string(raw))
	if !ok || !config.RevisionPattern.MatchString(rev) {
		return GitSnapshot{}
	}
	snapshot := GitSnapshot{Revision: Ptr(rev)}
	// Check index shortcuts that can hide subject changes from status.
	index, ok := run("ls-files", "-v", "-z")
	if !ok {
		return snapshot
	}
	allowStage := q.Config.SchemaVersion == config.TrustedSchema && q.Config.Providers.Invaris.Enabled
	for _, entry := range bytes.Split(bytes.TrimSuffix(index, []byte{0}), []byte{0}) {
		if len(entry) < 2 {
			continue
		}
		if entry[0] == 'S' || entry[0] >= 'a' && entry[0] <= 'z' {
			return snapshot
		}
		if allowStage && string(entry[2:]) == config.RuntimeHarness {
			snapshot.Dirty = Ptr(true)
			return snapshot
		}
	}
	status, ok := run("status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=all", "--ignored", "--ignore-submodules=none")
	if !ok {
		return snapshot
	}
	// Exempt exactly one launcher-provided ignored file. A tracked file or
	// another entry under .invaris remains candidate input.
	snapshot.Dirty = Ptr(false)
	for _, entry := range bytes.Split(bytes.TrimSuffix(status, []byte{0}), []byte{0}) {
		if len(entry) == 0 || allowStage && string(entry) == "!! "+config.RuntimeHarness {
			continue
		}
		snapshot.Dirty = Ptr(true)
		break
	}
	return snapshot
}
