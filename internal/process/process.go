// Package process executes trusted programs through a bounded, explicit boundary.
package process

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/akkaraponph/itervyn/internal/config"
)

type Spec struct {
	ExpectedSHA256           string
	Executable               string
	Args                     []string
	Dir                      string
	Env                      []string
	Timeout                  time.Duration
	StdoutLimit, StderrLimit int64
}
type Outcome struct {
	Stdout, Stderr                   []byte
	ExitCode                         *int
	Reason                           string
	StartedAt, FinishedAt            time.Time
	ExecutablePath, ExecutableSHA256 string
}
type Runner interface {
	Run(context.Context, Spec) Outcome
}

func ContextReason(ctx context.Context) string {
	if ctx.Err() == context.DeadlineExceeded {
		return "TIMEOUT"
	}
	return "CANCELED"
}

type Identity struct {
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Module   string `json:"module"`
	Version  string `json:"version"`
	Revision string `json:"revision"`
	Dirty    bool   `json:"dirty"`
}

func Resolve(executable string) (string, string, error) {
	p, err := exec.LookPath(executable)
	if err != nil {
		return "", "", config.Err("TOOL_MISSING", "executable unavailable")
	}
	p, err = filepath.Abs(p)
	if err != nil {
		return "", "", err
	}
	// Explicit executables must be regular files, not symlinks. PATH tools may
	// resolve through trusted system directory anchors, but not a linked leaf.
	st, err := os.Lstat(p)
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0111 == 0 {
		return "", "", config.Err("TOOL_MISSING", "regular executable required")
	}
	f, err := os.Open(p)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, 128<<20+1))
	if err != nil || n > 128<<20 {
		return "", "", config.Err("OUTPUT_LIMIT", "executable exceeds identity limit")
	}
	return p, fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}
func Identify(executable, provider string) (Identity, error) {
	var out Identity
	p, h, err := Resolve(executable)
	if err != nil {
		return out, err
	}
	out.Path, out.SHA256 = p, h
	bi, err := buildinfo.ReadFile(p)
	if err != nil {
		return out, config.Err("UNSUPPORTED_TOOL", "Go build identity unavailable")
	}
	out.Module, out.Version = bi.Main.Path, bi.Main.Version
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" {
			out.Revision = s.Value
		}
		if s.Key == "vcs.modified" {
			out.Dirty = s.Value == "true"
		}
	}
	if err = Supported(out, provider); err != nil {
		return out, err
	}
	return out, probeVersion(out, provider)
}

// probeVersion runs `<provider> --version` on the identified file and requires
// it to print exactly "<provider> <build version>". Both the static build
// identity and the release's own report must name the same supported release.
func probeVersion(id Identity, provider string) error {
	tmp, err := os.MkdirTemp("", "itervyn-version-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	o := (Local{}).Run(context.Background(), Spec{ExpectedSHA256: id.SHA256, Executable: id.Path, Args: []string{"--version"}, Dir: tmp, Env: []string{"PATH=" + os.Getenv("PATH"), "TMPDIR=" + tmp}, Timeout: 10 * time.Second, StdoutLimit: 256, StderrLimit: 4096})
	if o.Reason == "EVIDENCE_MISMATCH" || o.Reason == "TOOL_MISSING" {
		return config.Err(o.Reason, "provider changed during identification")
	}
	if o.Reason != "" || o.ExitCode == nil || *o.ExitCode != 0 || strings.TrimSuffix(string(o.Stdout), "\n") != provider+" "+id.Version {
		return config.Err("UNSUPPORTED_TOOL", "provider --version does not match its build identity")
	}
	return nil
}

// Supported reports whether a build identity is an accepted provider release.
func Supported(id Identity, provider string) error {
	lines, ok := SupportedReleases[provider]
	if !ok {
		return config.Err("UNSUPPORTED_TOOL", "unknown provider")
	}
	if id.Module != "github.com/akkaraponph/"+provider || id.Dirty || !slices.Contains(lines, ReleaseLine(id.Version)) {
		return config.Err("UNSUPPORTED_TOOL", "unsupported "+provider+" build, use a clean release on "+strings.Join(lines, " or "))
	}
	return nil
}

// SupportedReleases are the release lines (major.minor) whose wire contracts
// Itervyn implements, per provider. A binary is accepted only when its Go build
// identity names the provider's module at a clean, tagged release on one of
// these lines. Development builds, pseudo-versions, prereleases and other
// lines fail closed: their contracts have not been checked against Itervyn.
var SupportedReleases = map[string][]string{"invaris": {"0.5"}, "evaris": {"0.4", "0.5"}}

var release = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// ReleaseLine returns "major.minor" for a tagged release version such as
// v0.5.2, or "" for anything else ("(devel)", pseudo-versions, prereleases,
// build metadata).
func ReleaseLine(version string) string {
	m := release.FindStringSubmatch(version)
	if m == nil {
		return ""
	}
	return m[1] + "." + m[2]
}
func Environment(names []string, tmp string) []string {
	e := []string{"PATH=" + os.Getenv("PATH"), "TMPDIR=" + tmp}
	for _, n := range names {
		if v, ok := os.LookupEnv(n); ok {
			e = append(e, n+"="+v)
		}
	}
	return e
}

type Record struct {
	Identity         Identity  `json:"identity"`
	ArgvSHA256       string    `json:"argvSha256"`
	EnvironmentNames []string  `json:"environmentNames"`
	StartedAt        time.Time `json:"startedAt"`
	FinishedAt       time.Time `json:"finishedAt"`
	ExitCode         *int      `json:"exitCode"`
	Reason           string    `json:"reason"`
}

func RecordJSON(id Identity, s Spec, o Outcome) []byte {
	b, _ := json.Marshal(append([]string{s.Executable}, s.Args...))
	names := []string{}
	for _, e := range s.Env {
		names = append(names, strings.SplitN(e, "=", 2)[0])
	}
	r := Record{id, config.Hash(b), names, o.StartedAt, o.FinishedAt, o.ExitCode, o.Reason}
	b, _ = json.MarshalIndent(r, "", "  ")
	return append(b, '\n')
}

type bounded struct {
	mu       sync.Mutex
	data     []byte
	limit    int64
	exceeded bool
	cancel   context.CancelFunc
}

func (b *bounded) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - int64(len(b.data))
	if int64(len(p)) > remaining {
		b.data = append(b.data, p[:remaining]...)
		b.exceeded = true
		b.cancel()
	} else {
		b.data = append(b.data, p...)
	}
	return len(p), nil
}
func (b *bounded) snapshot() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte{}, b.data...), b.exceeded
}
