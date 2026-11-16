package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/evidence"
	"github.com/akkaraponph/itervyn/internal/verify"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func oneObject(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	d := json.NewDecoder(bytes.NewReader(b))
	if e := d.Decode(&m); e != nil {
		t.Fatal(e, string(b))
	}
	var rest any
	if e := d.Decode(&rest); e != io.EOF {
		t.Fatal("extra JSON/text", string(b))
	}
	return m
}

type failingProvider struct{ calls *int }

func (failingProvider) Name() string { return "invaris" }
func (p failingProvider) Verify(_ context.Context, q verify.Request) verify.Observation {
	*p.calls++
	return verify.Observation{Result: verify.ProviderResult{Name: "invaris", Status: verify.Fail, NativeDecision: verify.Ptr("REJECT"), NativeSchemaVersion: verify.Ptr("invaris.evidence/v1alpha2"), ExitCode: verify.Ptr(1), StartedAt: verify.Ptr(time.Now().UTC()), FinishedAt: verify.Ptr(time.Now().UTC()), ExecutableSHA256: verify.Ptr(config.Hash(nil)), NativeReasons: []string{}, Evidence: []verify.ArtifactRef{}, Scope: verify.Scope{Kind: "working-tree", ProjectRoot: verify.Ptr(q.Config.ProjectRoot)}}, Artifacts: []verify.NativeArtifact{{Path: "providers/invaris.json", Data: []byte(`{"synthetic":true}`)}}}
}
func TestCLIContracts(t *testing.T) {
	calls := 0
	s := verify.Service{Store: evidence.Store{}, Providers: []verify.Provider{failingProvider{&calls}}}
	path := filepath.Join(t.TempDir(), "itervyn.json")
	invoke := func(want int, args ...string) map[string]any {
		t.Helper()
		var out, err bytes.Buffer
		code := Run(context.Background(), args, &out, &err, s)
		if code != want {
			t.Fatalf("%v: exit %d want %d stdout=%s stderr=%s", args, code, want, out.String(), err.String())
		}
		return oneObject(t, out.Bytes())
	}
	if v := invoke(0, "version", "--format", "json"); v["version"] != "v0.1.0" || v["configSchemas"] == nil {
		t.Fatal(v)
	}
	invoke(0, "init", "--config", path, "--format", "json")
	before, _ := os.ReadFile(path)
	invoke(2, "init", "--config", path, "--format", "json")
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("init overwrite")
	}
	result := invoke(1, "verify", "--config", path, "--format", "json")
	id := result["runId"].(string)
	if calls != 1 {
		t.Fatal(calls)
	}
	invoke(1, "inspect", id, "--config", path, "--format", "json")
	if calls != 1 {
		t.Fatal("inspect executed provider")
	}
	invoke(2, "doctor", "--config", path, "--format", "json")
	if calls != 1 {
		t.Fatal("doctor executed provider")
	}
	for _, args := range [][]string{{"bad"}, {"verify", "--unknown"}, {"verify", "--config"}, {"verify", "--config", path, "--config", path}, {"inspect"}, {"version", "--config", path}, {"verify", "extra"}} {
		args = append(args, "--format", "json")
		if d := invoke(2, args...); d["code"] != "INVALID_ARGUMENT" {
			t.Fatal(d)
		}
	}
	_ = os.WriteFile(path, []byte(`{"providers":null}`), 0600)
	invoke(2, "verify", "--config", path, "--format", "json")
	if calls != 1 {
		t.Fatal("invalid configuration invoked provider")
	}
}
func TestSubprocessCommands(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "itervyn")
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin/go"), "build", "-trimpath", "-o", binary, "../../cmd/itervyn")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	configPath := filepath.Join(root, "itervyn.json")
	for _, tc := range []struct {
		args []string
		want int
	}{{[]string{"version", "--format", "json"}, 0}, {[]string{"init", "--config", configPath, "--format", "json"}, 0}, {[]string{"init", "--config", configPath, "--format", "json"}, 2}, {[]string{"doctor", "--config", configPath, "--format", "json"}, 2}, {[]string{"verify", "--config", configPath, "--format", "json"}, 2}, {[]string{"inspect", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--config", configPath, "--format", "json"}, 2}} {
		cmd := exec.Command(binary, tc.args...)
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				code = e.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if code != tc.want {
			t.Fatal(tc.args, code, stderr.String())
		}
		oneObject(t, out.Bytes())
	}
	cmd = exec.Command(binary, "--help")
	if b, e := cmd.CombinedOutput(); e != nil || !bytes.Contains(b, []byte("inspect")) {
		t.Fatal(e, string(b))
	}
	// An unrecognized executable is never invoked by doctor.
	sentinel := filepath.Join(root, "EXECUTED")
	script := filepath.Join(root, "fake-provider")
	_ = os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+sentinel+"'\n"), 0700)
	c := config.Starter()
	c.Providers.Invaris.Executable = script
	b, _ := json.Marshal(c)
	_ = os.WriteFile(configPath, b, 0600)
	cmd = exec.Command(binary, "doctor", "--config", configPath, "--format", "json")
	_ = cmd.Run()
	if _, e := os.Stat(sentinel); !os.IsNotExist(e) {
		t.Fatal("doctor executed unrecognized provider")
	}
}
