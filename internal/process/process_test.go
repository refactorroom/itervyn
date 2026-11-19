package process

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/akkaraponph/itervyn/internal/config"
)

func TestProcessHelper(t *testing.T) {
	if os.Getenv("ITERVYN_HELPER") != "1" {
		return
	}
	args := os.Args
	idx := 0
	for n, a := range args {
		if a == "--" {
			idx = n + 1
			break
		}
	}
	mode := args[idx]
	switch mode {
	case "args":
		_ = json.NewEncoder(os.Stdout).Encode(args[idx+1:])
	case "env":
		fmt.Print(os.Getenv("ITERVYN_SENTINEL"))
	case "stdout":
		fmt.Print(strings.Repeat("x", 20000))
	case "stderr":
		fmt.Fprint(os.Stderr, strings.Repeat("e", 20000))
	case "hang":
		fmt.Print(`{"status":"PASS"}`)
		for {
			time.Sleep(time.Second)
		}
	case "child":
		cmd := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$", "--", "hang")
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			os.Exit(3)
		}
		fmt.Printf("child=%d\n", cmd.Process.Pid)
		for {
			time.Sleep(time.Second)
		}
	case "exit":
		os.Exit(7)
	}
	os.Exit(0)
}
func spec(t *testing.T, mode string) Spec {
	return Spec{Executable: os.Args[0], Args: []string{"-test.run=^TestProcessHelper$", "--", mode}, Dir: t.TempDir(), Env: []string{"ITERVYN_HELPER=1"}, Timeout: 3 * time.Second, StdoutLimit: 1000, StderrLimit: 1000}
}
func TestExplicitProcess(t *testing.T) {
	s := spec(t, "args")
	s.Args = append(s.Args, "a b", "$(touch SHOULD_NOT_EXIST)", "x;y")
	o := (Local{}).Run(context.Background(), s)
	var args []string
	if o.Reason != "" || o.ExitCode == nil || *o.ExitCode != 0 || json.Unmarshal(o.Stdout, &args) != nil || len(args) != 3 || args[1] != "$(touch SHOULD_NOT_EXIST)" {
		t.Fatalf("%+v %s", o, o.Stdout)
	}
	t.Setenv("ITERVYN_SENTINEL", "private-value")
	s = spec(t, "env")
	o = (Local{}).Run(context.Background(), s)
	if len(o.Stdout) != 0 {
		t.Fatal("inherited secret")
	}
	s.Env = append(s.Env, Environment([]string{"ITERVYN_SENTINEL"}, t.TempDir())...)
	o = (Local{}).Run(context.Background(), s)
	if string(o.Stdout) != "private-value" {
		t.Fatal("allowlist not passed")
	}
}

func TestChangedExecutableDoesNotStart(t *testing.T) {
	_, expected, e := Resolve(os.Args[0])
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	sentinel := filepath.Join(root, "started")
	replacement := filepath.Join(root, "changed-binary")
	if e = os.WriteFile(replacement, []byte("#!/bin/sh\ntouch '"+sentinel+"'\n"), 0700); e != nil {
		t.Fatal(e)
	}
	s := spec(t, "args")
	s.Executable = replacement
	s.ExpectedSHA256 = expected
	o := (Local{}).Run(context.Background(), s)
	if o.Reason != "EVIDENCE_MISMATCH" || o.ExitCode != nil {
		t.Fatal(o)
	}
	if _, e = os.Stat(sentinel); !os.IsNotExist(e) {
		t.Fatal("replacement executed")
	}
}
func TestLimitsAndCancellation(t *testing.T) {
	for _, mode := range []string{"stdout", "stderr", "hang", "child"} {
		t.Run(mode, func(t *testing.T) {
			s := spec(t, mode)
			s.Timeout = 100 * time.Millisecond
			if mode == "stdout" || mode == "stderr" {
				s.Timeout = 3 * time.Second
			}
			start := time.Now()
			o := (Local{}).Run(context.Background(), s)
			want := "TIMEOUT"
			if mode == "stdout" || mode == "stderr" {
				want = "OUTPUT_LIMIT"
			}
			if o.Reason != want || len(o.Stdout) > 1000 || len(o.Stderr) > 1000 || time.Since(start) > 2500*time.Millisecond {
				t.Fatalf("%+v duration=%s", o, time.Since(start))
			}
			if mode == "stdout" && len(o.Stdout) != 1000 || mode == "stderr" && len(o.Stderr) != 1000 {
				t.Fatal("capture did not stop at the exact cap")
			}
			if mode == "child" {
				for _, line := range strings.Split(string(o.Stdout), "\n") {
					if strings.HasPrefix(line, "child=") {
						pid, e := strconv.Atoi(strings.TrimPrefix(line, "child="))
						if e != nil {
							t.Fatal(e)
						}
						if syscall.Kill(pid, 0) == nil {
							b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
							if !strings.Contains(string(b), ") Z ") {
								t.Fatal("descendant remained alive", pid)
							}
						}
					}
				}
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if o := (Local{}).Run(ctx, spec(t, "hang")); o.Reason != "CANCELED" || o.ExitCode != nil {
		t.Fatal(o)
	}
	s := spec(t, "hang")
	s.Executable = "/missing/itervyn-test"
	if o := (Local{}).Run(context.Background(), s); o.Reason != "TOOL_MISSING" {
		t.Fatal(o)
	}
	s = spec(t, "exit")
	o := (Local{}).Run(context.Background(), s)
	if o.ExitCode == nil || *o.ExitCode != 7 || o.Reason != "" {
		t.Fatal(o)
	}
}

func TestSupportedReleases(t *testing.T) {
	ok := []struct{ provider, version string }{
		{"invaris", "v0.5.0"},
		{"evaris", "v0.4.0"},
		{"evaris", "v0.5.0"},
	}
	for _, tc := range ok {
		provider := tc.provider
		id := Identity{Module: "github.com/akkaraponph/" + provider, Version: tc.version}
		if e := Supported(id, provider); e != nil {
			t.Fatal(provider, e)
		}
		patch := id
		patch.Version = strings.TrimSuffix(id.Version, "0") + "7"
		if e := Supported(patch, provider); e != nil {
			t.Fatal("patch release refused", patch, e)
		}
	}
	refused := []struct {
		provider string
		id       Identity
	}{
		{"invaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "v0.3.0"}},
		{"invaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "v0.4.0"}},
		{"invaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "v0.6.0"}},
		{"invaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "v1.5.0"}},
		{"invaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "(devel)", Revision: "ca79f9696f783094b6154f0c59414cff6ee483e6"}},
		{"invaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "v0.5.1-0.20261117072319-ca79f9696f78"}},
		{"invaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "v0.5.0-rc.1"}},
		{"invaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "v0.5.0+dirty"}},
		{"invaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "v0.05.0"}},
		{"invaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "v0.5.0", Dirty: true}},
		{"invaris", Identity{Module: "github.com/akkaraponph/evaris", Version: "v0.5.0"}},
		{"invaris", Identity{Module: "example.com/invaris", Version: "v0.5.0"}},
		{"evaris", Identity{Module: "github.com/akkaraponph/evaris", Version: "v0.2.0"}},
		{"evaris", Identity{Module: "github.com/akkaraponph/evaris", Version: "v0.6.0"}},
		{"evaris", Identity{Module: "github.com/akkaraponph/invaris", Version: "v0.4.0"}},
		{"other", Identity{Module: "github.com/akkaraponph/other", Version: "v0.5.0"}},
	}
	for _, tc := range refused {
		if e := Supported(tc.id, tc.provider); config.Code(e) != "UNSUPPORTED_TOOL" {
			t.Fatal("accepted", tc.provider, tc.id, e)
		}
	}
	// The test binary itself is a Go build of another module.
	if _, e := Identify(os.Args[0], "invaris"); config.Code(e) != "UNSUPPORTED_TOOL" {
		t.Fatal(e)
	}
	script := filepath.Join(t.TempDir(), "invaris")
	if e := os.WriteFile(script, []byte("#!/bin/sh\necho invaris v0.5.0\n"), 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := Identify(script, "invaris"); config.Code(e) != "UNSUPPORTED_TOOL" {
		t.Fatal("launcher script accepted", e)
	}
}

// fakeProvider builds a clean, tagged Go binary of the provider's module that
// prints say for --version.
func fakeProvider(t *testing.T, provider, tag, say string) string {
	t.Helper()
	if _, e := exec.LookPath("git"); e != nil {
		t.Skip("git unavailable")
	}
	src := t.TempDir()
	write := func(name, body string) {
		if e := os.WriteFile(filepath.Join(src, name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	write("go.mod", "module github.com/akkaraponph/"+provider+"\n\ngo 1.22\n")
	write("main.go", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println("+strconv.Quote(say)+") }\n")
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
		cmd.Dir = src
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatal(e, string(b))
		}
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "fake")
	git("tag", tag)
	bin := filepath.Join(t.TempDir(), provider)
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", bin, ".")
	cmd.Dir = src
	cmd.Env = append(os.Environ(), "GOFLAGS=-buildvcs=true", "GOTOOLCHAIN=local")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	return bin
}
func TestVersionProbe(t *testing.T) {
	bin := fakeProvider(t, "invaris", "v0.5.3", "invaris v0.5.3")
	id, e := Identify(bin, "invaris")
	if e != nil || id.Version != "v0.5.3" || id.Dirty {
		t.Fatal(id, e)
	}
	for name, bin := range map[string]string{
		"mismatched --version": fakeProvider(t, "invaris", "v0.5.3", "invaris v0.5.4"),
		"dev --version":        fakeProvider(t, "invaris", "v0.5.3", "invaris dev"),
		"other tool name":      fakeProvider(t, "invaris", "v0.5.3", "evaris v0.5.3"),
		"unsupported line":     fakeProvider(t, "evaris", "v0.6.0", "evaris v0.6.0"),
	} {
		provider := filepath.Base(bin)
		if _, e := Identify(bin, provider); config.Code(e) != "UNSUPPORTED_TOOL" {
			t.Fatal(name, "accepted", e)
		}
	}
}
