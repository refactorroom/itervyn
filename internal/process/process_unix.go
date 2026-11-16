//go:build linux || darwin

package process

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Local struct{}

func (Local) Run(parent context.Context, s Spec) (out Outcome) {
	out.StartedAt = time.Now().UTC()
	defer func() { out.FinishedAt = time.Now().UTC() }()
	if parent.Err() != nil {
		out.Reason = ContextReason(parent)
		return
	}
	if s.Timeout <= 0 || s.StdoutLimit <= 0 || s.StderrLimit <= 0 {
		out.Reason = "INTERNAL_ERROR"
		return
	}
	p, h, err := Resolve(s.Executable)
	if err != nil {
		out.Reason = "TOOL_MISSING"
		return
	}
	out.ExecutablePath, out.ExecutableSHA256 = p, h
	if s.ExpectedSHA256 != "" && s.ExpectedSHA256 != h {
		out.Reason = "EVIDENCE_MISMATCH"
		return
	}
	ctx, cancel := context.WithTimeout(parent, s.Timeout)
	defer cancel()
	stdout := &bounded{limit: s.StdoutLimit, cancel: cancel}
	stderr := &bounded{limit: s.StderrLimit, cancel: cancel}
	cmd := exec.CommandContext(ctx, p, s.Args...)
	cmd.Dir = s.Dir
	cmd.Env = append([]string{}, s.Env...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = time.Second
	var once sync.Once
	terminated := make(chan struct{})
	terminate := func() {
		once.Do(func() {
			if cmd.Process == nil {
				close(terminated)
				return
			}
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			go func() {
				timer := time.NewTimer(time.Second)
				defer timer.Stop()
				<-timer.C
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				close(terminated)
			}()
		})
	}
	cmd.Cancel = func() error { terminate(); return nil }
	if err = cmd.Start(); err != nil {
		out.Reason = "TOOL_MISSING"
		return
	}
	err = cmd.Wait()
	exit := cmd.ProcessState.ExitCode()
	out.ExitCode = &exit
	var exceededA, exceededB bool
	out.Stdout, exceededA = stdout.snapshot()
	out.Stderr, exceededB = stderr.snapshot()
	if ctx.Err() != nil || errors.Is(err, exec.ErrWaitDelay) {
		terminate()
		<-terminated
	}
	switch {
	case exceededA || exceededB:
		out.Reason = "OUTPUT_LIMIT"
	case parent.Err() != nil:
		out.Reason = ContextReason(parent)
	case ctx.Err() != nil || errors.Is(err, exec.ErrWaitDelay):
		out.Reason = "TIMEOUT"
	}
	_, after, e := Resolve(p)
	if e != nil || after != h {
		out.Reason = "EVIDENCE_MISMATCH"
	}
	return
}
