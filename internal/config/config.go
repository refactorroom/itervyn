// Package config loads a strict, non-executing JSON configuration.
package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

const Schema = "itervyn.config/v1alpha1"
const TrustedSchema = "itervyn.config/v1alpha2"

// RuntimeHarness is staged by the trusted launcher. Invaris 0.5 derives
// its target from the harness directory (or the parent of .invaris).
const RuntimeHarness = ".invaris/itervyn-harness.json"
const MaxConfig = 1 << 20

var RevisionPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
var IDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var DigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var EnvPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Failure struct {
	Code   string
	Detail string
}

func (e *Failure) Error() string    { return e.Code + ": " + e.Detail }
func Err(code, detail string) error { return &Failure{code, detail} }
func Code(err error) string {
	if e, ok := err.(*Failure); ok {
		return e.Code
	}
	return "IO_ERROR"
}
func Hash(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

type Project struct {
	Root             string `json:"root" optional:"true"`
	RequireCleanTree bool   `json:"requireCleanTree,omitempty" optional:"true"`
}
type Execution struct {
	TimeoutSeconds         int      `json:"timeoutSeconds" optional:"true"`
	ProviderTimeoutSeconds int      `json:"providerTimeoutSeconds" optional:"true"`
	StdoutLimitBytes       int64    `json:"stdoutLimitBytes" optional:"true"`
	StderrLimitBytes       int64    `json:"stderrLimitBytes" optional:"true"`
	EnvironmentAllowlist   []string `json:"environmentAllowlist" optional:"true"`
}
type Invaris struct {
	Enabled    bool   `json:"enabled"`
	Required   bool   `json:"required"`
	Executable string `json:"executable" optional:"true"`
	Action     string `json:"action" optional:"true"`
	Harness    string `json:"harness" optional:"true"`
}
type Evaris struct {
	Enabled    bool   `json:"enabled"`
	Required   bool   `json:"required"`
	Executable string `json:"executable" optional:"true"`
	Store      string `json:"store,omitempty" optional:"true"`
	RunID      string `json:"runId,omitempty" optional:"true"`
}
type Providers struct {
	Invaris Invaris `json:"invaris" optional:"true"`
	Evaris  Evaris  `json:"evaris" optional:"true"`
}
type Evidence struct {
	Directory string `json:"directory" optional:"true"`
}
type Config struct {
	SchemaVersion string    `json:"schemaVersion"`
	Project       Project   `json:"project" optional:"true"`
	Execution     Execution `json:"execution" optional:"true"`
	Providers     Providers `json:"providers"`
	Evidence      Evidence  `json:"evidence" optional:"true"`
}
type Resolved struct {
	Config
	ConfigPath, ProjectRoot, HarnessPath, EvidenceDir, EvarisStore string
	RuntimeHarnessPath                                             string
	ConfigBytes                                                    []byte
	ConfigSHA256                                                   string
}

func Defaults() Config {
	return Config{SchemaVersion: Schema, Project: Project{Root: "."}, Execution: Execution{300, 120, 4194304, 65536, []string{}}, Providers: Providers{Invaris: Invaris{Executable: "invaris", Action: "check", Harness: "invaris.json"}, Evaris: Evaris{Executable: "evaris"}}, Evidence: Evidence{".itervyn/runs"}}
}
func Starter() Config {
	c := Defaults()
	c.Providers.Invaris.Enabled = true
	c.Providers.Invaris.Required = true
	return c
}
func Load(path string) (Config, error) {
	c := Defaults()
	b, err := ReadAbsolute(path, MaxConfig)
	if err != nil {
		return c, err
	}
	if err = DecodeStrict(b, &c); err != nil {
		return c, Err("INVALID_CONFIG", "configuration must match the strict schema")
	}
	var raw struct {
		Project   map[string]json.RawMessage `json:"project"`
		Providers struct {
			Evaris map[string]any `json:"evaris"`
		} `json:"providers"`
	}
	_ = json.Unmarshal(b, &raw)
	if !c.Providers.Evaris.Enabled {
		for _, key := range []string{"store", "runId"} {
			if _, ok := raw.Providers.Evaris[key]; ok {
				return c, Err("INVALID_CONFIG", "disabled Evaris cannot have run context")
			}
		}
	}
	if _, present := raw.Project["requireCleanTree"]; present && c.SchemaVersion == Schema {
		return c, Err("INVALID_CONFIG", "requireCleanTree needs config v1alpha2")
	}
	if err = validate(c); err != nil {
		return c, err
	}
	return c, nil
}
func validate(c Config) error {
	bad := func(s string) error { return Err("INVALID_CONFIG", s) }
	e := c.Execution
	if e.TimeoutSeconds < 1 || e.TimeoutSeconds > 3600 || e.ProviderTimeoutSeconds < 1 || e.ProviderTimeoutSeconds > 3600 || e.StdoutLimitBytes < 1 || e.StdoutLimitBytes > 4194304 || e.StderrLimitBytes < 1 || e.StderrLimitBytes > 65536 {
		return bad("execution limits")
	}
	seen := map[string]bool{}
	for _, n := range e.EnvironmentAllowlist {
		if !EnvPattern.MatchString(n) || seen[n] || n == "PATH" || n == "TMPDIR" {
			return bad("environment allowlist")
		}
		seen[n] = true
	}
	i, v := c.Providers.Invaris, c.Providers.Evaris
	if i.Required && !i.Enabled || v.Required && !v.Enabled || !(i.Enabled && i.Required || v.Enabled && v.Required) {
		return bad("at least one enabled required provider is necessary")
	}
	for _, p := range []string{i.Executable, v.Executable} {
		if p == "" || strings.ContainsRune(p, 0) || !filepath.IsAbs(p) && strings.ContainsAny(p, `/\`) {
			return bad("executable must be a basename or absolute path")
		}
	}
	if i.Action != "check" {
		return Err("UNSUPPORTED_ACTION", "only Invaris check is supported")
	}
	switch c.SchemaVersion {
	case Schema:
		if !SafeRelative(i.Harness) || filepath.Base(i.Harness) != i.Harness || c.Project.RequireCleanTree {
			return bad("v1alpha1 requires a project-root harness basename")
		}
	case TrustedSchema:
		if !SafeRelative(i.Harness) {
			return bad("v1alpha2 requires a config-relative harness")
		}
	default:
		return Err("UNSUPPORTED_SCHEMA", "configuration version")
	}
	if v.Enabled {
		if !filepath.IsAbs(v.Store) || !SafeLexical(v.Store) || !IDPattern.MatchString(v.RunID) {
			return bad("Evaris needs an absolute store and explicit run ID")
		}
	} else if v.Store != "" || v.RunID != "" {
		return bad("disabled Evaris cannot have run context")
	}
	if c.Project.Root == "" || !SafeLexical(c.Project.Root) || !(SafeRelative(c.Evidence.Directory) || c.SchemaVersion == TrustedSchema && filepath.IsAbs(c.Evidence.Directory) && SafeLexical(c.Evidence.Directory)) {
		return Err("PATH_UNSAFE", "invalid root or evidence path")
	}
	return nil
}
func Resolve(c Config, configPath string) (Resolved, error) {
	r := Resolved{Config: c}
	if err := validate(c); err != nil {
		return r, err
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return r, err
	}
	anchor, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return r, Err("PATH_UNSAFE", "configuration directory unavailable")
	}
	abs = filepath.Join(anchor, filepath.Base(abs))
	r.ConfigPath = abs
	r.ConfigBytes, err = ReadAbsolute(abs, MaxConfig)
	if err != nil {
		return r, err
	}
	// Bind the accepted parsed values to exactly the bytes read at resolution.
	parsed := Defaults()
	if e := DecodeStrict(r.ConfigBytes, &parsed); e != nil {
		return r, Err("INVALID_CONFIG", "configuration changed")
	}
	if !equalConfig(c, parsed) {
		return r, Err("EVIDENCE_MISMATCH", "configuration changed")
	}
	r.ConfigSHA256 = Hash(r.ConfigBytes)
	root := c.Project.Root
	if !filepath.IsAbs(root) {
		root = filepath.Join(filepath.Dir(abs), root)
	}
	r.ProjectRoot, err = filepath.EvalSymlinks(root)
	if err != nil {
		return r, Err("PATH_UNSAFE", "project root unavailable")
	}
	st, err := os.Stat(r.ProjectRoot)
	if err != nil || !st.IsDir() {
		return r, Err("PATH_UNSAFE", "project root must be directory")
	}
	r.HarnessPath = filepath.Join(r.ProjectRoot, c.Providers.Invaris.Harness)
	r.RuntimeHarnessPath = r.HarnessPath
	if c.SchemaVersion == TrustedSchema {
		r.HarnessPath = filepath.Join(anchor, c.Providers.Invaris.Harness)
		r.RuntimeHarnessPath = filepath.Join(r.ProjectRoot, RuntimeHarness)
	}
	r.EvidenceDir = c.Evidence.Directory
	if !filepath.IsAbs(r.EvidenceDir) {
		r.EvidenceDir = filepath.Join(r.ProjectRoot, r.EvidenceDir)
	}
	r.EvidenceDir = filepath.Clean(r.EvidenceDir)
	if err = CheckAbsolute(r.EvidenceDir, true); err != nil {
		return r, err
	}
	if st, e := os.Stat(r.EvidenceDir); e == nil && !st.IsDir() {
		return r, Err("PATH_UNSAFE", "evidence destination must be directory")
	}
	if Within(r.HarnessPath, r.EvidenceDir) || Within(r.EvidenceDir, r.HarnessPath) || Within(r.EvidenceDir, r.RuntimeHarnessPath) {
		return r, Err("PATH_UNSAFE", "evidence overlaps harness")
	}
	if c.Providers.Evaris.Enabled {
		// A missing store is a provider prerequisite, rather than invalid JSON.
		r.EvarisStore = filepath.Clean(c.Providers.Evaris.Store)
		if actual, e := filepath.EvalSymlinks(r.EvarisStore); e == nil {
			r.EvarisStore = actual
		}
		if Within(r.EvarisStore, r.EvidenceDir) || Within(r.EvidenceDir, r.EvarisStore) {
			return r, Err("PATH_UNSAFE", "evidence overlaps Evaris store")
		}
	}
	return r, nil
}
func equalConfig(a, b Config) bool { return reflect.DeepEqual(a, b) }

// ReadHarness keeps the path anchored while rejecting linked descendants.
func (r Resolved) ReadHarness() ([]byte, error) {
	if r.SchemaVersion == TrustedSchema {
		return Read(filepath.Dir(r.ConfigPath), r.Providers.Invaris.Harness, MaxConfig)
	}
	return Read(r.ProjectRoot, r.Providers.Invaris.Harness, MaxConfig)
}

// ReadRuntimeHarness reads the exact candidate-relative location understood by
// Invaris. The caller's launcher must keep that location outside write reach.
func (r Resolved) ReadRuntimeHarness() ([]byte, error) {
	if r.SchemaVersion == TrustedSchema {
		return Read(r.ProjectRoot, RuntimeHarness, MaxConfig)
	}
	return r.ReadHarness()
}
