package verify_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/evidence"
	"github.com/akkaraponph/itervyn/internal/verify"
)

func gitRun(t *testing.T, root string, args ...string) string {
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

func fixtureFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func strictFixture(t *testing.T, noGit, importedOnly bool) (config.Resolved, string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "subject")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, filepath.Join(root, "source.go"), "package subject\n")
	fixtureFile(t, filepath.Join(root, ".gitignore"), "*.cache\n")
	revision := strings.Repeat("a", 40)
	if !noGit {
		gitRun(t, root, "init", "-q")
		gitRun(t, root, "add", ".")
		gitRun(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "fixture")
		revision = gitRun(t, root, "rev-parse", "HEAD")
	}
	c := config.Starter()
	c.SchemaVersion = config.TrustedSchema
	c.Project = config.Project{Root: root, RequireCleanTree: true}
	c.Evidence.Directory = filepath.Join(base, "runs")
	if importedOnly {
		c.Providers.Invaris.Enabled = false
		c.Providers.Invaris.Required = false
		c.Providers.Evaris = config.Evaris{Enabled: true, Required: true, Executable: "evaris", Store: filepath.Join(base, "import"), RunID: strings.Repeat("b", 32)}
	}
	path := filepath.Join(base, "itervyn.json")
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, path, string(b))
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := config.Resolve(loaded, path)
	if err != nil {
		t.Fatal(err)
	}
	return r, revision
}

type observedProvider struct {
	provider
	git        *verify.GitSnapshot
	incomplete bool
	change     func()
}

func (p observedProvider) Verify(ctx context.Context, q verify.Request) verify.Observation {
	if p.change != nil {
		p.change()
	}
	o := p.provider.Verify(ctx, q)
	o.Git, o.Incomplete = p.git, p.incomplete
	return o
}

func TestRevisionGate(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		want       verify.Status
	}{
		{"clean", "", verify.Pass},
		{"no git", "GIT_UNAVAILABLE", verify.Inconclusive},
		{"expected absent", "REVISION_REQUIRED", verify.Inconclusive},
		{"mismatch", "REVISION_MISMATCH", verify.Inconclusive},
		{"tracked", "GIT_DIRTY", verify.Inconclusive},
		{"hidden in index", "GIT_STATUS_UNAVAILABLE", verify.Inconclusive},
		{"untracked", "GIT_DIRTY", verify.Inconclusive},
		{"ignored", "GIT_DIRTY", verify.Inconclusive},
		{"drift during provider", "GIT_DIRTY", verify.Inconclusive},
		{"moved HEAD", "REVISION_MOVED", verify.Inconclusive},
		{"missing native revision", "PROVENANCE_CONFLICT", verify.Inconclusive},
		{"dirty native", "PROVENANCE_CONFLICT", verify.Inconclusive},
		{"conflicting native", "PROVENANCE_CONFLICT", verify.Inconclusive},
		{"incomplete", "EVIDENCE_INCOMPLETE", verify.Inconclusive},
		{"import only", "LIVE_EVIDENCE_REQUIRED", verify.Inconclusive},
		{"known failure", "GIT_DIRTY", verify.Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, revision := strictFixture(t, tc.name == "no git", tc.name == "import only")
			root := c.ProjectRoot
			p := observedProvider{provider: provider{name: "invaris", status: verify.Pass}, git: &verify.GitSnapshot{Revision: verify.Ptr(revision), Dirty: verify.Ptr(false)}}
			expect := revision
			switch tc.name {
			case "expected absent":
				expect = ""
			case "mismatch":
				expect = strings.Repeat("9", len(revision))
			case "tracked":
				fixtureFile(t, filepath.Join(root, "source.go"), "package edited\n")
			case "hidden in index":
				gitRun(t, root, "update-index", "--assume-unchanged", "source.go")
				fixtureFile(t, filepath.Join(root, "source.go"), "package edited\n")
			case "untracked":
				fixtureFile(t, filepath.Join(root, "stray"), "untracked")
			case "ignored":
				fixtureFile(t, filepath.Join(root, "stray.cache"), "ignored")
			case "drift during provider":
				p.change = func() { fixtureFile(t, filepath.Join(root, "generated.cache"), "ignored drift") }
			case "moved HEAD":
				p.change = func() {
					gitRun(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--allow-empty", "-qm", "moved")
				}
			case "missing native revision":
				p.git.Revision = nil
			case "dirty native":
				p.git.Dirty = verify.Ptr(true)
			case "conflicting native":
				p.git.Revision = verify.Ptr(strings.Repeat("8", len(revision)))
			case "incomplete":
				p.incomplete = true
			case "import only":
				p = observedProvider{provider: provider{name: "evaris", status: verify.Pass}}
			case "known failure":
				p.provider.status = verify.Fail
				fixtureFile(t, filepath.Join(root, "stray.cache"), "ignored")
			}
			s := verify.Service{Providers: []verify.Provider{p}, Store: evidence.Store{}}
			out, err := s.Verify(context.Background(), verify.Request{Config: c, ExpectedRevision: expect})
			if err != nil || out.Status != tc.want || out.SchemaVersion != verify.GatedResultSchema || !out.EvidencePersisted {
				t.Fatalf("want %s, got %+v, %v", tc.want, out, err)
			}
			if tc.code != "" && !slices.ContainsFunc(out.Feedback, func(f verify.Feedback) bool { return f.Code == tc.code }) {
				t.Fatalf("%s missing from %v", tc.code, out.Feedback)
			}
			inspected, err := s.Store.Inspect(context.Background(), c.EvidenceDir, out.RunID)
			if err != nil || inspected.Status != tc.want {
				t.Fatal(inspected, err)
			}
			if tc.want == verify.Inconclusive {
				out.Status, out.NextAction = verify.Pass, verify.Complete
				if verify.ValidateResult(out) == nil {
					t.Fatal("published result validator accepted a false PASS")
				}
			}
		})
	}
}
