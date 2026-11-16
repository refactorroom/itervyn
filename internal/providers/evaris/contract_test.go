package evaris

import (
	"context"
	"encoding/json"
	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/process"
	"github.com/akkaraponph/itervyn/internal/verify"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func evaluation() Evaluation {
	return Evaluation{SchemaVersion: EvaluationSchema, ID: strings.Repeat("e", 32), RunID: strings.Repeat("a", 32), PreparationID: strings.Repeat("b", 32), ExperimentID: "experiment", CaseID: "sample", ArmID: "baseline", RunDigest: config.Hash([]byte("run")), OracleDigest: config.Hash([]byte("oracle")), CreatedAt: time.Now().UTC(), Execution: "COMPLETE", AgentOutcome: "SATISFIED", Graders: []Grader{{CriterionID: "exact", Required: true, Status: "PASS", Evidence: []FileRef{}, GraderVersion: "builtin/1"}}, Review: "NOT_REQUIRED", Assessment: "PASS", Limitations: []string{}}
}
func TestEvaluationContract(t *testing.T) {
	for _, status := range []string{"PASS", "FAIL", "INCONCLUSIVE"} {
		e := evaluation()
		e.Assessment = status
		code := map[string]int{"PASS": 0, "FAIL": 1, "INCONCLUSIVE": 2}[status]
		b, _ := json.Marshal(e)
		if _, err := parse(b, e.RunID, &code); err != nil {
			t.Fatal(err)
		}
	}
	for i, mut := range []func(*Evaluation){func(e *Evaluation) { e.RunID = "wrong" }, func(e *Evaluation) { e.PreparationID = "wrong" }, func(e *Evaluation) { e.ID = "../escape" }, func(e *Evaluation) { e.RunDigest = "wrong" }, func(e *Evaluation) { e.Graders = nil }, func(e *Evaluation) { e.Graders = append(e.Graders, e.Graders[0]) }, func(e *Evaluation) { e.Graders[0].Status = "MAYBE" }, func(e *Evaluation) { e.SchemaVersion = "unknown" }, func(e *Evaluation) {
		e.Verification = &Verification{Status: "invalid", Evidence: []FileRef{}, Findings: []Finding{}}
	}} {
		e := evaluation()
		mut(&e)
		b, _ := json.Marshal(e)
		if _, err := parse(b, strings.Repeat("a", 32), verify.Ptr(0)); err == nil {
			t.Fatal("mutation accepted", i)
		}
	}
	e := evaluation()
	b, _ := json.Marshal(e)
	if _, err := parse(b, e.RunID, verify.Ptr(2)); config.Code(err) != "EXIT_MISMATCH" {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{nil, []byte(`{}`), []byte(strings.Replace(string(b), `"assessment":`, `"assessment":"PASS","assessment":`, 1)), []byte(strings.Replace(string(b), `"review":`, `"unknown":true,"review":`, 1))} {
		if _, err := parse(bad, e.RunID, verify.Ptr(0)); err == nil {
			t.Fatal("malformed accepted")
		}
	}
}

type runFunc func(context.Context, process.Spec) process.Outcome

func (f runFunc) Run(ctx context.Context, s process.Spec) process.Outcome { return f(ctx, s) }
func TestExistingRunAdapter(t *testing.T) {
	store := t.TempDir()
	e := evaluation()
	for _, dir := range []string{"", filepath.Join("runs", e.RunID), filepath.Join("evaluations", e.ID)} {
		_ = os.MkdirAll(filepath.Join(store, dir), 0700)
		_ = os.WriteFile(filepath.Join(store, dir, "COMMITTED"), []byte("marker"), 0600)
	}
	b, _ := json.MarshalIndent(e, "", "  ")
	b = append(b, '\n')
	_ = os.WriteFile(filepath.Join(store, "evaluations", e.ID, "evaluation.json"), b, 0600)
	c := config.Resolved{Config: config.Starter(), ProjectRoot: t.TempDir(), EvarisStore: store}
	c.Providers.Evaris = config.Evaris{Enabled: true, Required: true, Executable: "evaris", Store: store, RunID: e.RunID}
	id := process.Identity{Path: "/trusted/evaris", SHA256: config.Hash(nil)}
	called := 0
	a := Adapter{Identify: func(string, string) (process.Identity, error) { return id, nil }, Runner: runFunc(func(_ context.Context, s process.Spec) process.Outcome {
		called++
		if len(s.Args) != 5 || s.Args[0] != "evaluate" || s.Args[1] != "--store" || s.Args[3] != "--run" || s.Args[4] != e.RunID {
			t.Fatal(s.Args)
		}
		return process.Outcome{Stdout: b, ExitCode: verify.Ptr(0), ExecutableSHA256: id.SHA256}
	})}
	out := a.Verify(context.Background(), verify.Request{Config: c})
	if out.Result.Status != verify.Pass || *out.Result.Scope.EvaluationID != e.ID || len(out.Artifacts) != 4 {
		t.Fatalf("%+v", out)
	}
	_ = os.WriteFile(filepath.Join(store, "evaluations", e.ID, "evaluation.json"), []byte("corrupt"), 0600)
	if out = a.Verify(context.Background(), verify.Request{Config: c}); *out.Result.Reason != "EVIDENCE_MISMATCH" {
		t.Fatal(out)
	}
	_ = os.Remove(filepath.Join(store, "runs", e.RunID, "COMMITTED"))
	if out = a.Verify(context.Background(), verify.Request{Config: c}); *out.Result.Reason != "PREREQUISITE_MISSING" || called != 2 {
		t.Fatal(out, called)
	}
}
