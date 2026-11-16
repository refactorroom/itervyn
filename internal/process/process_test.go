package process

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
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
