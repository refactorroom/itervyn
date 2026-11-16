package invaris

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

const harness = `{"version":"invaris.dev/v1alpha2","name":"test","trustMode":"trusted-local","networkPolicy":"unrestricted","policies":[{"id":"rule","allowEmpty":false}],"gate":{"requiredGuards":["g"],"requiredProofs":[],"precedence":"failure-first","allowWeakEvidence":false},"guards":[{"id":"g","policyId":"rule","program":"guard"}]}`

func report(t *testing.T, decision string) (Harness, Report) {
	t.Helper()
	h, e := ParseHarness([]byte(harness))
	if e != nil {
		t.Fatal(e)
	}
	r := Report{SchemaVersion: EvidenceSchema, RunID: strings.Repeat("a", 32), Action: "check", Target: "/project", Harness: h.Name, ConfigDigest: verify.Ptr(config.Hash([]byte(harness))), StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(), Policies: h.Policies, Gate: h.Gate, Steps: []Check{}, Proofs: []json.RawMessage{}, Artifacts: []json.RawMessage{}, Diagnostics: []Diagnostic{}, TargetCount: verify.Ptr(int64(1)), Decision: decision}
	r.Environment.OS = "darwin"
	r.Environment.Arch = "arm64"
	r.Compatibility.Status = "native"
	r.Compatibility.InputVersion = HarnessSchema
	r.Compatibility.Warnings = []string{}
	c := Check{ID: "g", PolicyID: "rule", ProviderID: "invaris-command", Status: "PASS", Inspected: verify.Ptr(int64(1)), ExitCode: verify.Ptr(0), Findings: []Finding{}, Diagnostics: []Diagnostic{}}
	if decision == "REJECT" {
		c.Status = "FAIL"
		c.ExitCode = verify.Ptr(1)
		c.Findings = append(c.Findings, Finding{PolicyID: "rule", RuleID: "rule", Path: "source.txt"})
	}
	if decision == "REFUSE" {
		c.Status = "REFUSE"
		c.ExitCode = verify.Ptr(2)
	}
	r.Checks = []Check{c}
	r.Lifecycle = []Event{{ID: "g", Kind: "guard", Status: c.Status}}
	o := Observation{ID: "g", Kind: "guard", Program: "guard", Args: []string{}, Scope: []string{}, Diagnostics: []Diagnostic{}, EnvAllowlist: []string{}}
	o.Provider.Name = "invaris-command"
	r.Observations = []Observation{o}
	return h, r
}
func TestNativeContract(t *testing.T) {
	t.Run("configured inspection floor", func(t *testing.T) {
		_, r := report(t, "ACCEPT")
		b := []byte(strings.Replace(harness, `"program":"guard"`, `"program":"guard","minInspected":2`, 1))
		h, err := ParseHarness(b)
		if err != nil {
			t.Fatal(err)
		}
		r.ConfigDigest = verify.Ptr(config.Hash(b))
		encoded, _ := json.Marshal(r)
		if _, err = validateReport(encoded, h, r.Target, *r.ConfigDigest, verify.Ptr(0)); config.Code(err) != "OUTPUT_INVALID" {
			t.Fatal("configured inspection floor ignored", err)
		}
	})
	for _, d := range []string{"ACCEPT", "REJECT", "REFUSE"} {
		h, r := report(t, d)
		b, _ := json.Marshal(r)
		code := map[string]int{"ACCEPT": 0, "REJECT": 1, "REFUSE": 2}[d]
		if _, e := validateReport(b, h, r.Target, *r.ConfigDigest, &code); e != nil {
			t.Fatal(d, e)
		}
	}
	mutations := []func(*Report){func(r *Report) { r.SchemaVersion = "invaris.evidence/v1alpha5" }, func(r *Report) { r.Target = "/wrong" }, func(r *Report) { r.Harness = "wrong" }, func(r *Report) { r.ConfigDigest = verify.Ptr(config.Hash(nil)) }, func(r *Report) { r.Checks = nil }, func(r *Report) { r.Checks = append(r.Checks, r.Checks[0]) }, func(r *Report) { r.Checks[0].Inspected = verify.Ptr(int64(0)) }, func(r *Report) { r.TargetCount = nil }, func(r *Report) { r.Lifecycle[0].ID = "wrong" }, func(r *Report) { r.Checks[0].Findings = []Finding{{PolicyID: "rule"}} }, func(r *Report) { r.Observations = []Observation{} }, func(r *Report) { r.Compatibility.Status = "legacy" }}
	for i, mut := range mutations {
		h, r := report(t, "ACCEPT")
		mut(&r)
		b, _ := json.Marshal(r)
		if _, e := validateReport(b, h, "/project", config.Hash([]byte(harness)), verify.Ptr(0)); e == nil {
			t.Fatal("accepted mutation", i)
		}
	}
	h, r := report(t, "ACCEPT")
	b, _ := json.Marshal(r)
	for _, bad := range [][]byte{nil, []byte("garbage"), []byte(strings.Replace(string(b), `"checks":`, `"unknown":true,"checks":`, 1)), []byte(strings.Replace(string(b), `"decision":`, `"decision":"ACCEPT","decision":`, 1))} {
		if _, e := validateReport(bad, h, r.Target, *r.ConfigDigest, verify.Ptr(0)); e == nil {
			t.Fatal("malformed accepted")
		}
	}
	if _, e := validateReport(b, h, r.Target, *r.ConfigDigest, verify.Ptr(1)); config.Code(e) != "EXIT_MISMATCH" {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(harness, `"allowEmpty":false`, `"allowEmpty":true`, 1), strings.Replace(harness, `"requiredGuards":["g"]`, `"requiredGuards":[]`, 1), strings.Replace(harness, `"failure-first"`, `"refusal-first"`, 1), strings.Replace(harness, `"program":"guard"`, `"program":"guard","adapterId":"x"`, 1)} {
		if _, e := ParseHarness([]byte(bad)); e == nil {
			t.Fatal("invalid profile accepted")
		}
	}
}

type runFunc func(context.Context, process.Spec) process.Outcome

func (f runFunc) Run(ctx context.Context, s process.Spec) process.Outcome { return f(ctx, s) }
func TestAdapterFailureBoundary(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	path := filepath.Join(root, "invaris.json")
	_ = os.WriteFile(path, []byte(harness), 0600)
	c := config.Resolved{Config: config.Starter(), ProjectRoot: root, HarnessPath: path}
	_, r := report(t, "ACCEPT")
	r.Target = root
	b, _ := json.Marshal(r)
	count := 0
	id := process.Identity{Path: "/trusted/invaris", SHA256: config.Hash([]byte("binary"))}
	a := Adapter{Identify: func(string, string) (process.Identity, error) { return id, nil }, Runner: runFunc(func(_ context.Context, s process.Spec) process.Outcome {
		count++
		if strings.Join(s.Args, " ") != "check --format json "+path {
			t.Fatal(s.Args)
		}
		return process.Outcome{Stdout: b, ExitCode: verify.Ptr(0), ExecutableSHA256: id.SHA256}
	})}
	q := verify.Request{Config: c}
	out := a.Verify(context.Background(), q)
	if out.Result.Status != verify.Pass || len(out.Artifacts) != 3 || count != 1 {
		t.Fatalf("%+v", out)
	}
	a.Runner = runFunc(func(context.Context, process.Spec) process.Outcome {
		return process.Outcome{Stdout: b, ExitCode: verify.Ptr(0), Reason: "TIMEOUT"}
	})
	if out = a.Verify(context.Background(), q); out.Result.Status != verify.Error || *out.Result.Reason != "TIMEOUT" {
		t.Fatal(out)
	}
	a.Runner = runFunc(func(context.Context, process.Spec) process.Outcome {
		_ = os.WriteFile(path, []byte(harness+" "), 0600)
		return process.Outcome{Stdout: b, ExitCode: verify.Ptr(0), ExecutableSHA256: id.SHA256}
	})
	if out = a.Verify(context.Background(), q); *out.Result.Reason != "EVIDENCE_MISMATCH" {
		t.Fatal(out)
	}
	_ = os.Remove(path)
	if out = a.Verify(context.Background(), q); *out.Result.Reason != "HARNESS_MISSING" {
		t.Fatal(out)
	}
}
