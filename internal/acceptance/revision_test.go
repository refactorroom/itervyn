package acceptance

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/akkaraponph/itervyn/internal/config"
	invaris "github.com/akkaraponph/itervyn/internal/providers/invaris"
	"github.com/akkaraponph/itervyn/internal/verify"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}

func commitFixture(t *testing.T, root string) string {
	t.Helper()
	git(t, root, "add", ".")
	git(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "fixture")
	return git(t, root, "rev-parse", "HEAD")
}

func requireOutcome(t *testing.T, r verify.Result, want verify.Status, code string) {
	t.Helper()
	if r.Status != want || !r.EvidencePersisted || r.RevisionGate == nil ||
		code != "" && !slices.ContainsFunc(r.Feedback, func(f verify.Feedback) bool { return f.Code == code }) {
		t.Fatalf("want %s (%s): got %+v", want, code, r)
	}
}

func TestRealRevisionGateArchitecture(t *testing.T) {
	bin := tool(t, "invaris")
	policy, root := temp(t), temp(t)
	guard := filepath.Join(policy, "verify-imports")
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin/go"), "build", "-trimpath", "-o", guard, "../../.itervyn-policy/guard")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(b))
	}
	harness, err := os.ReadFile("../../.itervyn-policy/invaris.json")
	if err != nil {
		t.Fatal(err)
	}
	harness = []byte(strings.ReplaceAll(string(harness), "/trusted/bin/verify-imports", guard))
	write(t, filepath.Join(policy, "invaris.json"), harness)
	stage := filepath.Join(root, config.RuntimeHarness)
	source := filepath.Join(root, "internal/verify/source.go")
	write(t, source, []byte("package verify\n"))
	write(t, filepath.Join(root, ".gitignore"), []byte("*.cache\n"))
	git(t, root, "init", "-q")
	prepared := commitFixture(t, root)
	write(t, filepath.Join(root, ".git/info/exclude"), []byte("/"+config.RuntimeHarness+"\n"))
	write(t, stage, harness)

	c := config.Starter()
	c.SchemaVersion = config.TrustedSchema
	c.Project = config.Project{Root: root, RequireCleanTree: true}
	c.Evidence.Directory = filepath.Join(policy, "runs")
	c.Providers.Invaris.Executable = bin

	clean := runWithPolicy(t, policy, c, prepared)
	requireOutcome(t, clean, verify.Pass, "")
	if clean.RevisionGate.Invaris == nil || *clean.RevisionGate.Invaris.Revision != prepared || *clean.RevisionGate.Invaris.Dirty {
		t.Fatal("native provenance was lost")
	}
	raw, err := os.ReadFile(filepath.Join(c.Evidence.Directory, clean.RunID, "providers/invaris.json"))
	var native invaris.Report
	if err != nil || json.Unmarshal(raw, &native) != nil || native.ConfigDigest == nil ||
		*native.ConfigDigest != config.Hash(harness) || native.Target != root {
		t.Fatal("native target/digest did not match trusted input", err)
	}

	stray := filepath.Join(root, "stray.cache")
	write(t, stray, []byte("a pre-existing ignored input"))
	ignored := runWithPolicy(t, policy, c, prepared)
	requireOutcome(t, ignored, verify.Inconclusive, "GIT_DIRTY")
	if ignored.Providers[0].Status != verify.Pass {
		t.Fatal("ignored-input case did not exercise the gate independently of Invaris")
	}
	if err = os.Remove(stray); err != nil {
		t.Fatal(err)
	}

	write(t, stage, append(append([]byte{}, harness...), ' '))
	requireOutcome(t, runWithPolicy(t, policy, c, prepared), verify.Inconclusive, "EVIDENCE_MISMATCH")
	write(t, stage, harness)

	write(t, source, []byte("package verify\nimport _ `github.com/akkaraponph/itervyn/internal/providers/invaris`\n"))
	// These candidate files attempt to remove the real guard from the gate.
	write(t, filepath.Join(root, "itervyn.json"), []byte(`{"providers":{"invaris":{"enabled":false,"required":false}}}`))
	write(t, filepath.Join(root, "invaris.json"), []byte(`{"guards":[],"gate":{"requiredGuards":[]}}`))
	bad := commitFixture(t, root)
	failed := runWithPolicy(t, policy, c, bad)
	requireOutcome(t, failed, verify.Fail, "")
	raw, err = os.ReadFile(filepath.Join(c.Evidence.Directory, failed.RunID, "providers/invaris.json"))
	native = invaris.Report{}
	if err != nil || json.Unmarshal(raw, &native) != nil || len(native.Checks) != 1 ||
		len(native.Checks[0].Findings) != 1 || native.Checks[0].Findings[0].Path != "internal/verify/source.go" {
		t.Fatal("architecture rejection evidence unavailable", err)
	}
}

func TestRealRevisionGateRejectsImportedOnly(t *testing.T) {
	bin := tool(t, "evaris")
	nativeRoot, id := prepareImport(t, bin, "pass")
	root, policy := temp(t), temp(t)
	write(t, filepath.Join(root, "source.go"), []byte("package subject\n"))
	git(t, root, "init", "-q")
	revision := commitFixture(t, root)

	c := config.Defaults()
	c.SchemaVersion = config.TrustedSchema
	c.Project = config.Project{Root: root, RequireCleanTree: true}
	c.Evidence.Directory = filepath.Join(policy, "runs")
	c.Providers.Evaris = config.Evaris{Enabled: true, Required: true, Executable: bin, Store: filepath.Join(nativeRoot, "store"), RunID: id}
	r := runWithPolicy(t, policy, c, revision)
	requireOutcome(t, r, verify.Inconclusive, "LIVE_EVIDENCE_REQUIRED")
	if len(r.Providers) != 1 || r.Providers[0].Status != verify.Pass || r.Providers[0].Scope.Kind != "imported-run" {
		t.Fatal("imported PASS was not recorded separately")
	}
}
