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

// Evaris 0.4 adds an optional verification message and finding line.
func TestVerificationAdditions(t *testing.T) {
	e := evaluation()
	e.Assessment, e.Execution, e.AgentOutcome = "FAIL", "COMPLETE", "VIOLATED"
	e.Verification = &Verification{Required: true, Status: "REJECT", Evidence: []FileRef{}, Findings: []Finding{{PolicyID: "p", RuleID: "r", Path: "a.go", Line: verify.Ptr(4)}}, Message: "Invaris rejected the delivered tree."}
	b, _ := json.Marshal(e)
	if _, err := parse(b, e.RunID, verify.Ptr(1)); err != nil {
		t.Fatal(err)
	}
	legacy := strings.Replace(strings.Replace(string(b), `"line":4,`, ``, 1), `"message":"Invaris rejected the delivered tree.",`, ``, 1)
	if strings.Contains(legacy, `"line"`) || strings.Count(legacy, `"message"`) != 2 {
		t.Fatal("fixture still carries additions", legacy)
	}
	if _, err := parse([]byte(legacy), e.RunID, verify.Ptr(1)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(string(b), `"line":4`, `"line":0`, 1), strings.Replace(string(b), `"line":4`, `"line":4,"column":1`, 1), strings.Replace(string(b), `"message":"Invaris`, `"message":5,"x":"Invaris`, 1)} {
		if _, err := parse([]byte(bad), e.RunID, verify.Ptr(1)); config.Code(err) != "OUTPUT_INVALID" {
			t.Fatal("accepted", err, bad)
		}
	}
}

func TestRevisionDiagnosticPolicy(t *testing.T) {
	e := Evaluation{
		Execution:    "INCOMPLETE",
		Review:       "NOT_REQUIRED",
		Limitations:  []string{"synthetic import"},
		Graders:      []Grader{{Required: true, Status: "PASS"}, {Status: "UNAVAILABLE", Reason: "COMMAND_NOT_AUTHORIZED"}},
		Verification: &Verification{Status: "UNAVAILABLE", Reason: "COMMAND_NOT_AUTHORIZED"},
	}
	if e.incompleteEvidence() {
		t.Fatal("an optional Evaris diagnostic blocked the gate")
	}
	e.Verification.Required = true
	if !e.incompleteEvidence() {
		t.Fatal("unavailable required verification accepted")
	}
	e.Verification.Required = false
	e.Graders[1].Required = true
	if !e.incompleteEvidence() {
		t.Fatal("unavailable required criterion accepted")
	}
	e.Graders[1].Required = false
	e.Execution = "CANCELED"
	if !e.incompleteEvidence() {
		t.Fatal("canceled evidence accepted")
	}
}
