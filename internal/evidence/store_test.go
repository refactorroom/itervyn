package evidence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/verify"
)

func setup(t *testing.T) (config.Resolved, verify.RunHandle, verify.Result) {
	t.Helper()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	c := config.Resolved{Config: config.Starter(), ProjectRoot: root, EvidenceDir: filepath.Join(root, ".itervyn/runs")}
	id := strings.Repeat("a", 32)
	h, e := (Store{}).Begin(context.Background(), c, id)
	if e != nil {
		t.Fatal(e)
	}
	r := verify.Result{SchemaVersion: verify.ResultSchema, RunID: id, Status: verify.Inconclusive, NextAction: verify.Investigate, Providers: []verify.ProviderResult{{Name: "invaris", Required: true, Status: verify.Error, Reason: verify.Ptr("TOOL_MISSING"), NativeReasons: []string{}, Evidence: []verify.ArtifactRef{}, Scope: verify.Scope{Kind: "working-tree", ProjectRoot: verify.Ptr(root)}}}, Feedback: []verify.Feedback{}, Warnings: []verify.Feedback{}, StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(), ConfigSHA256: verify.Ptr(config.Hash(nil)), ProjectRoot: verify.Ptr(root), Artifacts: []verify.ArtifactRef{}, EvidencePersisted: true}
	return c, h, r
}
func TestImmutableAndTamper(t *testing.T) {
	ctx := context.Background()
	s := Store{}
	c, h, r := setup(t)
	a, e := s.Put(ctx, h, verify.NativeArtifact{Path: "config.json", Data: []byte("original")})
	if e != nil {
		t.Fatal(e)
	}
	r.Artifacts = append(r.Artifacts, a)
	if _, e = s.Put(ctx, h, verify.NativeArtifact{Path: "config.json", Data: []byte("overwrite")}); e == nil {
		t.Fatal("overwritten")
	}
	if e = s.Commit(ctx, h, r); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(h.Directory, "result.json"))
	st, _ := os.Stat(filepath.Join(h.Directory, "result.json"))
	if _, e = s.Inspect(ctx, c.EvidenceDir, h.RunID); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(filepath.Join(h.Directory, "result.json"))
	now, _ := os.Stat(filepath.Join(h.Directory, "result.json"))
	if string(before) != string(after) || !st.ModTime().Equal(now.ModTime()) {
		t.Fatal("inspect modified result")
	}
	if _, e = s.Put(ctx, h, verify.NativeArtifact{Path: "new", Data: nil}); e == nil {
		t.Fatal("committed run changed")
	}
	if e = os.WriteFile(filepath.Join(h.Directory, "config.json"), []byte("tampered"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Inspect(ctx, c.EvidenceDir, h.RunID); config.Code(e) != "EVIDENCE_CORRUPT" {
		t.Fatal("corruption accepted", e)
	}
}
func TestIncompleteCollisionsAndUnsafeArtifacts(t *testing.T) {
	ctx := context.Background()
	s := Store{}
	c, h, r := setup(t)
	if _, e := s.Inspect(ctx, c.EvidenceDir, h.RunID); config.Code(e) != "INCOMPLETE_RUN" {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if _, e := s.Begin(ctx, c, h.RunID); e == nil {
				t.Error("duplicate reservation succeeded")
			}
		})
	}
	wg.Wait()
	for _, p := range []string{"../outside", "/absolute", "COMMITTED", "result.json"} {
		if _, e := s.Put(ctx, h, verify.NativeArtifact{Path: p}); e == nil {
			t.Fatal("unsafe path accepted", p)
		}
	}
	if e := os.Symlink(t.TempDir(), filepath.Join(h.Directory, "linked")); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Put(ctx, h, verify.NativeArtifact{Path: "linked/escape"}); e == nil {
		t.Fatal("symlink accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if e := s.Commit(canceled, h, r); e == nil {
		t.Fatal("canceled commit accepted")
	}
	// Result publication collision must leave the run incomplete.
	_ = os.WriteFile(filepath.Join(h.Directory, "result.json"), []byte("occupied"), 0600)
	if e := s.Commit(ctx, h, r); e == nil {
		t.Fatal("result overwritten")
	}
	if _, e := s.Inspect(ctx, c.EvidenceDir, h.RunID); config.Code(e) != "INCOMPLETE_RUN" {
		t.Fatal(e)
	}
}
func TestMarkerAndResultCorruption(t *testing.T) {
	for _, mode := range []string{"marker", "result", "schema", "identity", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			s := Store{}
			c, h, r := setup(t)
			if e := s.Commit(ctx, h, r); e != nil {
				t.Fatal(e)
			}
			b, _ := os.ReadFile(filepath.Join(h.Directory, "result.json"))
			switch mode {
			case "marker":
				_ = os.WriteFile(filepath.Join(h.Directory, "COMMITTED"), []byte("bad"), 0600)
			case "result":
				_ = os.WriteFile(filepath.Join(h.Directory, "result.json"), []byte("bad"), 0600)
			default:
				if mode == "schema" {
					r.SchemaVersion = "unknown"
				}
				if mode == "identity" {
					r.RunID = strings.Repeat("b", 32)
				}
				b, _ = json.Marshal(r)
				if mode == "duplicate" {
					b = []byte(strings.Replace(string(b), `"status":`, `"status":"PASS","status":`, 1))
				}
				_ = os.WriteFile(filepath.Join(h.Directory, "result.json"), b, 0600)
				_ = os.WriteFile(filepath.Join(h.Directory, "COMMITTED"), []byte(config.Hash(b)+"\n"), 0600)
			}
			if _, e := s.Inspect(ctx, c.EvidenceDir, h.RunID); e == nil {
				t.Fatal("tampered result accepted")
			}
		})
	}
}

func TestErrorStatusDoesNotBypassRecordValidation(t *testing.T) {
	for name, mutate := range map[string]func(*verify.Result){
		"provider": func(r *verify.Result) { r.Providers[0].Name = "unknown" },
		"status":   func(r *verify.Result) { r.Providers[0].Status = "MAYBE" },
		"reason":   func(r *verify.Result) { r.Providers[0].Reason = verify.Ptr("NOT_A_CODE") },
		"digest":   func(r *verify.Result) { r.Providers[0].ExecutableSHA256 = verify.Ptr("invalid") },
		"revision": func(r *verify.Result) { r.GitRevision = verify.Ptr("not-a-revision") },
		"scope":    func(r *verify.Result) { r.Providers[0].Scope.ProjectRoot = verify.Ptr("relative") },
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s := Store{}
			c, h, r := setup(t)
			r.Status, r.NextAction = verify.Error, verify.Stop
			if e := s.Commit(ctx, h, r); e != nil {
				t.Fatal("valid error record rejected", e)
			}
			mutate(&r)
			if e := verify.ValidateResult(r); e == nil {
				t.Fatal("invalid record accepted before persistence")
			}
			b, _ := json.Marshal(r)
			_ = os.WriteFile(filepath.Join(h.Directory, "result.json"), b, 0600)
			_ = os.WriteFile(filepath.Join(h.Directory, "COMMITTED"), []byte(config.Hash(b)+"\n"), 0600)
			if _, e := s.Inspect(ctx, c.EvidenceDir, r.RunID); e == nil {
				t.Fatal("schema-invalid error record accepted")
			}
		})
	}
}
