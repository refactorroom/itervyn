package acceptance

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/evidence"
	"github.com/akkaraponph/itervyn/internal/process"
	evarisprovider "github.com/akkaraponph/itervyn/internal/providers/evaris"
	invarisprovider "github.com/akkaraponph/itervyn/internal/providers/invaris"
	"github.com/akkaraponph/itervyn/internal/verify"
)

func temp(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func write(t *testing.T, path string, b []byte) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func jsonFile(t *testing.T, path string, v any) {
	t.Helper()
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	write(t, path, append(b, '\n'))
}
func tool(t *testing.T, name string) string {
	t.Helper()
	p := os.Getenv("ITERVYN_TEST_" + strings.ToUpper(name))
	if p == "" {
		t.Skip("real provider unavailable: ITERVYN_TEST_" + strings.ToUpper(name))
	}
	id, e := process.Identify(p, name)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("real %s path=%s sha256=%s revision=%s version=%s", name, id.Path, id.SHA256, id.Revision, id.Version)
	return p
}
func native(t *testing.T, exe string, args ...string) map[string]any {
	t.Helper()
	cmd := exec.Command(exe, args...)
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("native %s %v: %v\n%s", filepath.Base(exe), args, e, b)
	}
	var out map[string]any
	if e = json.Unmarshal(b, &out); e != nil {
		t.Fatal(e, string(b))
	}
	return out
}
func run(t *testing.T, root string, c config.Config) verify.Result {
	t.Helper()
	path := filepath.Join(root, "itervyn.json")
	jsonFile(t, path, c)
	loaded, e := config.Load(path)
	if e != nil {
		t.Fatal(e)
	}
	resolved, e := config.Resolve(loaded, path)
	if e != nil {
		t.Fatal(e)
	}
	s := verify.Service{Providers: []verify.Provider{invarisprovider.Adapter{}, evarisprovider.Adapter{}}, Store: evidence.Store{}}
	r, e := s.Verify(context.Background(), verify.Request{Config: resolved})
	if e != nil {
		t.Fatal(e)
	}
	inspected, e := s.Store.Inspect(context.Background(), resolved.EvidenceDir, r.RunID)
	if e != nil || inspected.Status != r.Status {
		t.Fatal(e)
	}
	if dir := os.Getenv("ITERVYN_EVIDENCE_DIR"); dir != "" {
		persist := filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "-"), r.RunID)
		e = filepath.WalkDir(filepath.Join(resolved.EvidenceDir, r.RunID), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(filepath.Join(resolved.EvidenceDir, r.RunID), p)
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			dest := filepath.Join(persist, rel)
			if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
				return err
			}
			return os.WriteFile(dest, b, 0600)
		})
		if e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("run=%s status=%s providers=%d", r.RunID, r.Status, len(r.Providers))
	return r
}
func TestRealInvarisCorrection(t *testing.T) {
	bin := tool(t, "invaris")
	root := temp(t)
	guard := filepath.Join(root, "guard-bin")
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin/go"), "build", "-trimpath", "-o", guard, "../../examples/go-project/guard")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	b, e := os.ReadFile("../../examples/go-project/invaris.json")
	if e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(root, "invaris.json"), b)
	c := config.Starter()
	c.Providers.Invaris.Executable = bin
	var ids []string
	for _, tc := range []struct {
		source string
		want   verify.Status
	}{{"FORBIDDEN", verify.Fail}, {"SAFE", verify.Pass}, {"REFUSE", verify.Inconclusive}} {
		write(t, filepath.Join(root, "app.go"), []byte("package example\nconst Message = \""+tc.source+"\"\n"))
		r := run(t, root, c)
		if r.Status != tc.want {
			t.Fatalf("want %s got %+v", tc.want, r)
		}
		ids = append(ids, r.RunID)
	}
	if ids[0] == ids[1] || ids[1] == ids[2] {
		t.Fatal("run overwritten")
	}
	write(t, filepath.Join(root, ".itervyn/runs", ids[1], "providers/invaris.json"), []byte("corrupt"))
	if _, e = (evidence.Store{}).Inspect(context.Background(), filepath.Join(root, ".itervyn/runs"), ids[1]); config.Code(e) != "EVIDENCE_CORRUPT" {
		t.Fatal(e)
	}
}
func prepareImport(t *testing.T, bin, mode string) (string, string) {
	t.Helper()
	root := temp(t)
	write(t, filepath.Join(root, "public/input.txt"), []byte("seed\n"))
	write(t, filepath.Join(root, "task.txt"), []byte("Write result.txt\n"))
	write(t, filepath.Join(root, "instruction.txt"), []byte("Follow acceptance criteria.\n"))
	criteria := []any{map[string]any{"id": "result", "kind": "exact", "required": true, "path": "result.txt", "expected": "approved\n"}}
	if mode == "command" {
		criteria = append(criteria, map[string]any{"id": "command", "kind": "command-exit", "required": true, "expected": 0, "command": map[string]any{"program": "/bin/sh", "args": []string{"-c", "touch " + filepath.Join(root, "UNAUTHORIZED")}, "envAllowlist": []string{}}})
	}
	jsonFile(t, filepath.Join(root, "reviewer/oracle.json"), map[string]any{"schemaVersion": "evaris.oracle/v1alpha1", "criteria": criteria})
	caseDef := map[string]any{"id": "sample", "fixture": "public", "task": "task.txt", "oracle": "reviewer/oracle.json", "sourceRevision": "initial", "allowedChanges": []string{"result.txt"}, "requiresReview": mode == "review"}
	if mode == "verification" {
		h := map[string]any{"version": "invaris.dev/v1alpha2", "name": "embedded", "trustMode": "trusted-local", "networkPolicy": "unrestricted", "policies": []any{map[string]any{"id": "result", "allowEmpty": false}}, "guards": []any{map[string]any{"id": "result", "policyId": "result", "program": "/bin/true", "args": []string{}}}, "gate": map[string]any{"requiredGuards": []string{"result"}, "requiredProofs": []string{}, "precedence": "refusal-first", "allowWeakEvidence": false}}
		jsonFile(t, filepath.Join(root, "reviewer/invaris.json"), h)
		caseDef["verification"] = map[string]any{"harness": "reviewer/invaris.json", "required": true, "timeoutSeconds": 10}
	}
	experiment := map[string]any{"schemaVersion": "evaris.experiment/v1alpha1", "id": "control", "revision": "initial", "cases": []any{caseDef}, "arms": []any{map[string]any{"id": "baseline", "instructions": []any{}}, map[string]any{"id": "specialist", "instructions": []any{map[string]any{"path": "instruction.txt", "revision": "initial"}}}}, "outputRoot": "store", "constraints": map[string]any{"tools": "local", "permissions": "workspace", "budget": "synthetic-control"}}
	p := filepath.Join(root, "experiment.json")
	jsonFile(t, p, experiment)
	native(t, bin, "validate", "--experiment", p)
	prep := native(t, bin, "prepare", "--experiment", p, "--tasks-root", filepath.Join(root, "tasks"))
	task := prep["tasks"].([]any)[0].(map[string]any)
	workspace := filepath.Join(root, "tasks", task["workspace"].(string))
	result := "approved\n"
	if mode == "fail" {
		result = "wrong\n"
	}
	write(t, filepath.Join(workspace, "result.txt"), []byte(result))
	write(t, filepath.Join(root, "response.txt"), []byte("Synthetic deterministic control; no agent session.\n"))
	observed := map[string]any{}
	for _, k := range []string{"host", "model", "modelSettings", "tools", "permissions", "budget", "contextId", "freshContext", "startedAt", "finishedAt", "loadedInstructions", "agentClaim"} {
		observed[k] = nil
	}
	request := map[string]any{"schemaVersion": "evaris.import/v1alpha1", "preparationId": prep["id"], "caseId": task["caseId"], "armId": task["armId"], "configDigest": prep["configDigest"], "fixtureDigest": task["fixtureDigest"], "sourceRevision": "initial", "finalResponse": "response.txt", "finalTree": workspace, "transcripts": []string{}, "diff": nil, "observed": observed, "limitations": []string{"Authored synthetic control, not a real model measurement."}, "origin": "synthetic-control"}
	jsonFile(t, filepath.Join(root, "import.json"), request)
	imported := native(t, bin, "import", "--store", filepath.Join(root, "store"), "--record", filepath.Join(root, "import.json"))
	return root, imported["id"].(string)
}
func TestRealEvarisExistingImport(t *testing.T) {
	bin := tool(t, "evaris")
	for _, mode := range []string{"pass", "fail", "review", "command", "verification"} {
		t.Run(mode, func(t *testing.T) {
			nativeRoot, id := prepareImport(t, bin, mode)
			project := temp(t)
			c := config.Defaults()
			c.Providers.Evaris = config.Evaris{Enabled: true, Required: true, Executable: bin, Store: filepath.Join(nativeRoot, "store"), RunID: id}
			r := run(t, project, c)
			want := verify.Pass
			if mode == "fail" {
				want = verify.Fail
			} else if mode != "pass" {
				want = verify.Inconclusive
			}
			if r.Status != want {
				t.Fatalf("want %s got %+v", want, r)
			}
			if r.Providers[0].Scope.Kind != "imported-run" || *r.Providers[0].Scope.RunID != id {
				t.Fatal("scope lost")
			}
			if mode == "pass" {
				first := *r.Providers[0].Scope.EvaluationID
				oldPath := filepath.Join(nativeRoot, "store/evaluations", first, "evaluation.json")
				before, _ := os.ReadFile(oldPath)
				again := run(t, project, c)
				after, _ := os.ReadFile(oldPath)
				if *again.Providers[0].Scope.EvaluationID == first || string(before) != string(after) {
					t.Fatal("native evaluation overwritten")
				}
				write(t, filepath.Join(nativeRoot, "store/runs", id, "run.json"), []byte("corrupt"))
				bad := run(t, project, c)
				if bad.Status != verify.Inconclusive || bad.Providers[0].Status != verify.Error {
					t.Fatal("corrupt native store accepted")
				}
			}
			if mode == "command" {
				if _, e := os.Stat(filepath.Join(nativeRoot, "UNAUTHORIZED")); !os.IsNotExist(e) {
					t.Fatal("command grader ran without authorization")
				}
			}
		})
	}
}

func TestRealCombinedRequiredOptional(t *testing.T) {
	i, e := tool(t, "invaris"), tool(t, "evaris")
	nativeRoot, runID := prepareImport(t, e, "pass")
	root := temp(t)
	guard := filepath.Join(root, "guard-bin")
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin/go"), "build", "-trimpath", "-o", guard, "../../examples/go-project/guard")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(b))
	}
	b, err := os.ReadFile("../../examples/go-project/invaris.json")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "invaris.json"), b)
	c := config.Starter()
	c.Providers.Invaris.Executable = i
	c.Providers.Evaris = config.Evaris{Enabled: true, Required: true, Executable: e, Store: filepath.Join(nativeRoot, "store"), RunID: runID}
	write(t, filepath.Join(root, "app.go"), []byte("package example\nconst Message = \"FORBIDDEN\"\n"))
	if r := run(t, root, c); r.Status != verify.Fail || r.Providers[1].Status != verify.Pass {
		t.Fatal("passing Evaris overrode mandatory live failure")
	}
	write(t, filepath.Join(root, "app.go"), []byte("package example\nconst Message = \"SAFE\"\n"))
	c.Providers.Evaris.RunID = strings.Repeat("f", 32)
	if r := run(t, root, c); r.Status != verify.Inconclusive || r.Providers[0].Status != verify.Pass {
		t.Fatal("missing required import passed")
	}
	c.Providers.Evaris.Required = false
	if r := run(t, root, c); r.Status != verify.Pass || len(r.Warnings) == 0 {
		t.Fatal("optional prerequisite became mandatory")
	}
	c.Providers.Invaris.Required = false
	c.Providers.Invaris.Executable = "/missing/invaris"
	c.Providers.Evaris.Required = true
	c.Providers.Evaris.RunID = runID
	if r := run(t, root, c); r.Status != verify.Pass || r.Providers[0].Status != verify.Error {
		t.Fatal("optional missing executable became a gate")
	}
}
