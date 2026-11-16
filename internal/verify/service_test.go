package verify_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/evidence"
	"github.com/akkaraponph/itervyn/internal/verify"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type provider struct {
	name   string
	status verify.Status
	order  *[]string
}

func (p provider) Name() string { return p.name }
func (p provider) Verify(_ context.Context, q verify.Request) verify.Observation {
	if p.order != nil {
		*p.order = append(*p.order, p.name)
	}
	now := time.Now().UTC()
	r := verify.ProviderResult{Name: p.name, Status: p.status, NativeReasons: []string{}, Evidence: []verify.ArtifactRef{}, StartedAt: &now, FinishedAt: &now, ExecutableSHA256: verify.Ptr(config.Hash(nil))}
	if p.name == "invaris" {
		r.Scope = verify.Scope{Kind: "working-tree", ProjectRoot: verify.Ptr(q.Config.ProjectRoot)}
		r.NativeSchemaVersion = verify.Ptr("invaris.evidence/v1alpha2")
		native := "ACCEPT"
		if p.status == verify.Fail {
			native = "REJECT"
		}
		if p.status == verify.Inconclusive {
			native = "REFUSE"
		}
		r.NativeDecision = &native
	} else {
		r.Scope = verify.Scope{Kind: "imported-run", Store: verify.Ptr(q.Config.EvarisStore), RunID: verify.Ptr(q.Config.Providers.Evaris.RunID), PreparationID: verify.Ptr("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), EvaluationID: verify.Ptr("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"), ExperimentID: verify.Ptr("experiment"), CaseID: verify.Ptr("case"), ArmID: verify.Ptr("baseline"), RunDigest: verify.Ptr(config.Hash(nil)), OracleDigest: verify.Ptr(config.Hash(nil))}
		r.NativeSchemaVersion = verify.Ptr("evaris.evaluation/v1alpha1")
		r.NativeDecision = verify.Ptr(string(p.status))
	}
	r.ExitCode = verify.Ptr(verify.Exit(p.status))
	if p.status == verify.Error {
		r.NativeDecision = nil
		r.NativeSchemaVersion = nil
		r.Reason = verify.Ptr("TOOL_MISSING")
	}
	return verify.Observation{Result: r, Artifacts: []verify.NativeArtifact{{Path: "providers/" + p.name + ".json", Data: []byte(`{"synthetic":true}`)}}}
}
func resolved(t *testing.T) config.Resolved {
	t.Helper()
	root := t.TempDir()
	c := config.Starter()
	c.Providers.Evaris = config.Evaris{Enabled: true, Required: true, Executable: "evaris", Store: filepath.Join(root, "native-store"), RunID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	b, _ := json.Marshal(c)
	p := filepath.Join(root, "itervyn.json")
	_ = os.WriteFile(p, b, 0600)
	loaded, e := config.Load(p)
	if e != nil {
		t.Fatal(e)
	}
	r, e := config.Resolve(loaded, p)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestOrchestration(t *testing.T) {
	for _, tc := range []struct {
		a, b     verify.Status
		optional bool
		want     verify.Status
	}{{verify.Fail, verify.Error, false, verify.Fail}, {verify.Pass, verify.Error, false, verify.Inconclusive}, {verify.Pass, verify.Error, true, verify.Pass}} {
		t.Run(string(tc.want), func(t *testing.T) {
			c := resolved(t)
			c.Providers.Evaris.Required = !tc.optional
			order := []string{}
			s := verify.Service{Providers: []verify.Provider{provider{"evaris", tc.b, &order}, provider{"invaris", tc.a, &order}}, Store: evidence.Store{}}
			r, e := s.Verify(context.Background(), verify.Request{Config: c})
			if e != nil || r.Status != tc.want || !r.EvidencePersisted || !reflect.DeepEqual(order, []string{"invaris", "evaris"}) {
				t.Fatalf("%+v %v order %v", r, e, order)
			}
			read, e := s.Store.Inspect(context.Background(), c.EvidenceDir, r.RunID)
			if e != nil || read.Status != r.Status {
				t.Fatal(read, e)
			}
		})
	}
}

type failingStore struct {
	evidence.Store
	begins  int
	collide bool
}

func (s *failingStore) Begin(ctx context.Context, c config.Resolved, id string) (verify.RunHandle, error) {
	s.begins++
	if s.collide {
		return verify.RunHandle{}, os.ErrExist
	}
	return s.Store.Begin(ctx, c, id)
}
func (*failingStore) Commit(context.Context, verify.RunHandle, verify.Result) error {
	return errors.New("injected publication failure")
}
func TestPublicationAndCollisionFailure(t *testing.T) {
	for _, collision := range []bool{false, true} {
		c := resolved(t)
		store := &failingStore{collide: collision}
		s := verify.Service{Providers: []verify.Provider{provider{"invaris", verify.Pass, nil}, provider{"evaris", verify.Pass, nil}}, Store: store}
		r, e := s.Verify(context.Background(), verify.Request{Config: c})
		if e == nil || r.Status != verify.Error || r.EvidencePersisted {
			t.Fatal(r, e)
		}
		if collision && store.begins != 3 {
			t.Fatal("collision retry count", store.begins)
		}
	}
}

type cancelStore struct {
	evidence.Store
	cancel       context.CancelFunc
	duringCommit bool
}

func (s cancelStore) Put(ctx context.Context, h verify.RunHandle, a verify.NativeArtifact) (verify.ArtifactRef, error) {
	ref, err := s.Store.Put(ctx, h, a)
	if a.Path == "processes.json" && !s.duringCommit {
		s.cancel()
	}
	return ref, err
}
func (s cancelStore) Commit(ctx context.Context, h verify.RunHandle, r verify.Result) error {
	if s.duringCommit {
		s.cancel()
	}
	return s.Store.Commit(ctx, h, r)
}
func TestCancellationDuringPublicationCannotPass(t *testing.T) {
	for _, duringCommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "processes", true: "commit"}[duringCommit], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := resolved(t)
			store := cancelStore{Store: evidence.Store{}, cancel: cancel, duringCommit: duringCommit}
			s := verify.Service{Providers: []verify.Provider{provider{"invaris", verify.Pass, nil}, provider{"evaris", verify.Pass, nil}}, Store: store}
			r, err := s.Verify(ctx, verify.Request{Config: c})
			if r.Status != verify.Error || r.NextAction != verify.Stop {
				t.Fatal("cancellation published success", r, err)
			}
			read, e := store.Inspect(context.Background(), c.EvidenceDir, r.RunID)
			if e == nil && read.Status == verify.Pass {
				t.Fatal("committed PASS after cancellation")
			}
			if !duringCommit && (err != nil || e != nil || !r.EvidencePersisted) {
				t.Fatal("explicit canceled record not retained", err, e)
			}
		})
	}
}

type cancelingProvider struct {
	provider
	cancel context.CancelFunc
}

func (p cancelingProvider) Verify(ctx context.Context, q verify.Request) verify.Observation {
	out := p.provider.Verify(ctx, q)
	p.cancel()
	return out
}
func TestNativeInconclusiveAtCancellationIsRetained(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := resolved(t)
	s := verify.Service{Providers: []verify.Provider{provider{"invaris", verify.Pass, nil}, cancelingProvider{provider{"evaris", verify.Inconclusive, nil}, cancel}}, Store: evidence.Store{}}
	r, err := s.Verify(ctx, verify.Request{Config: c})
	if err != nil || r.Status != verify.Inconclusive || r.NextAction != verify.Stop || !r.EvidencePersisted {
		t.Fatal(r, err)
	}
	if read, e := s.Store.Inspect(context.Background(), c.EvidenceDir, r.RunID); e != nil || read.Status != verify.Inconclusive || read.NextAction != verify.Stop {
		t.Fatal(read, e)
	}
}
