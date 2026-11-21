package invaris

// Wire DTOs follow the Apache-2.0 harness/evidence contracts of the Invaris
// release lines in process.SupportedReleases (0.5.x, source v0.5.0). No guard
// implementation is copied.
import (
	"encoding/json"
	"github.com/akkaraponph/itervyn/internal/config"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const HarnessSchema = "invaris.dev/v1alpha2"
const EvidenceSchema = "invaris.evidence/v1alpha2"

type Policy struct {
	ID         string `json:"id"`
	AllowEmpty bool   `json:"allowEmpty"`
}
type Gate struct {
	RequiredGuards    []string `json:"requiredGuards"`
	RequiredProofs    []string `json:"requiredProofs"`
	Precedence        string   `json:"precedence"`
	AllowWeakEvidence bool     `json:"allowWeakEvidence"`
}
type Guard struct {
	ID             string   `json:"id"`
	Program        string   `json:"program"`
	PolicyID       string   `json:"policyId"`
	Args           []string `json:"args" optional:"true"`
	EnvAllowlist   []string `json:"envAllowlist" optional:"true"`
	TimeoutSeconds int      `json:"timeoutSeconds" optional:"true"`
	MinInspected   int      `json:"minInspected" optional:"true"`
}
type Harness struct {
	Version              string            `json:"version"`
	Name                 string            `json:"name"`
	TrustMode            string            `json:"trustMode"`
	NetworkPolicy        string            `json:"networkPolicy"`
	WritablePaths        []string          `json:"writablePaths" optional:"true"`
	FailFast             bool              `json:"failFast" optional:"true"`
	AlwaysRunGuards      bool              `json:"alwaysRunGuards" optional:"true"`
	Policies             []Policy          `json:"policies"`
	Gate                 Gate              `json:"gate"`
	Guards               []Guard           `json:"guards"`
	RedactEnv            []string          `json:"redactEnv" optional:"true"`
	StoreSensitiveOutput bool              `json:"storeSensitiveOutput" optional:"true"`
	Adapters             []json.RawMessage `json:"adapters" optional:"true"`
	RequiredAdapters     []string          `json:"requiredAdapters" optional:"true"`
	Steps                []json.RawMessage `json:"steps" optional:"true"`
	Challenges           []json.RawMessage `json:"challenges" optional:"true"`
}

func ParseHarness(b []byte) (Harness, error) {
	var h Harness
	bad := func() (Harness, error) { return h, config.Err("INVALID_CONFIG", "unsupported Invaris check profile") }
	if len(b) > config.MaxConfig || config.DecodeStrict(b, &h) != nil {
		return bad()
	}
	if h.Version != HarnessSchema {
		return h, config.Err("UNSUPPORTED_SCHEMA", "Invaris harness version")
	}
	if strings.TrimSpace(h.Name) == "" || h.TrustMode != "trusted-local" || h.NetworkPolicy != "unrestricted" || len(h.Policies) == 0 || len(h.Guards) == 0 || len(h.Guards) > 128 || len(h.Adapters)+len(h.RequiredAdapters)+len(h.Steps)+len(h.Challenges) != 0 || len(h.Gate.RequiredProofs) != 0 || h.Gate.Precedence != "failure-first" || h.Gate.AllowWeakEvidence {
		return bad()
	}
	policies := map[string]bool{}
	guards := map[string]bool{}
	for _, p := range h.Policies {
		if strings.TrimSpace(p.ID) == "" || policies[p.ID] || p.AllowEmpty {
			return bad()
		}
		policies[p.ID] = true
	}
	for _, g := range h.Guards {
		if strings.TrimSpace(g.ID) == "" || guards[g.ID] || !policies[g.PolicyID] || strings.TrimSpace(g.Program) == "" || g.TimeoutSeconds < 0 || g.TimeoutSeconds > 300 || g.MinInspected < 0 {
			return bad()
		}
		guards[g.ID] = true
		for _, n := range g.EnvAllowlist {
			if n == "" || strings.ContainsAny(n, "=\x00") {
				return bad()
			}
		}
	}
	if len(h.Gate.RequiredGuards) != len(guards) {
		return bad()
	}
	seen := map[string]bool{}
	for _, id := range h.Gate.RequiredGuards {
		if !guards[id] || seen[id] {
			return bad()
		}
		seen[id] = true
	}
	for _, p := range h.WritablePaths {
		if !config.SafeRelative(p) {
			return bad()
		}
	}
	seen = map[string]bool{}
	for _, n := range h.RedactEnv {
		if n == "" || strings.ContainsAny(n, "=\x00") || seen[n] {
			return bad()
		}
		seen[n] = true
	}
	// Native minInspected must be positive when explicitly supplied.
	var raw struct {
		Guards []map[string]json.RawMessage `json:"guards"`
	}
	_ = json.Unmarshal(b, &raw)
	for i, m := range raw.Guards {
		if _, ok := m["minInspected"]; ok && h.Guards[i].MinInspected < 1 {
			return bad()
		}
	}
	return h, nil
}

type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Finding struct {
	PolicyID string `json:"policyId"`
	RuleID   string `json:"ruleId"`
	Path     string `json:"path"`
	// Line is the 1-based line a structured guard reported (Invaris 0.5+).
	Line    *int   `json:"line" optional:"true"`
	Message string `json:"message"`
}
type Check struct {
	ID          string       `json:"id"`
	PolicyID    string       `json:"policyId"`
	ProviderID  string       `json:"providerId"`
	Status      string       `json:"status"`
	Inspected   *int64       `json:"inspected"`
	Findings    []Finding    `json:"findings"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	ExitCode    *int         `json:"exitCode"`
}
type Event struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}
type Executable struct {
	Path   string  `json:"path"`
	SHA256 *string `json:"sha256"`
}
type Observation struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Program  string   `json:"program"`
	Args     []string `json:"args"`
	Provider struct {
		Name        string  `json:"name"`
		Version     *string `json:"version"`
		ToolName    *string `json:"toolName"`
		ToolVersion *string `json:"toolVersion"`
	} `json:"provider"`
	Scope          []string     `json:"scope"`
	StartedAt      *time.Time   `json:"startedAt"`
	FinishedAt     *time.Time   `json:"finishedAt"`
	ExitCode       *int         `json:"exitCode"`
	Stdout         string       `json:"stdout"`
	Stderr         string       `json:"stderr"`
	Truncated      bool         `json:"truncated"`
	Diagnostics    []Diagnostic `json:"diagnostics"`
	Executable     *Executable  `json:"executable"`
	TimeoutSeconds *int         `json:"timeoutSeconds"`
	EnvAllowlist   []string     `json:"envAllowlist"`
}
type Report struct {
	SchemaVersion string  `json:"schemaVersion"`
	RunID         string  `json:"runId"`
	Action        string  `json:"action"`
	Target        string  `json:"target"`
	Harness       string  `json:"harness"`
	ConfigDigest  *string `json:"configDigest"`
	GitRevision   *string `json:"gitRevision"`
	GitDirty      *bool   `json:"gitDirty"`
	Environment   struct {
		OS   string `json:"os"`
		Arch string `json:"arch"`
	} `json:"environment"`
	StartedAt     time.Time `json:"startedAt"`
	FinishedAt    time.Time `json:"finishedAt"`
	DurationMs    int64     `json:"durationMs"`
	Compatibility struct {
		Status       string   `json:"status"`
		InputVersion string   `json:"inputVersion"`
		Warnings     []string `json:"warnings"`
	} `json:"compatibility"`
	Policies     []Policy          `json:"policies"`
	Gate         Gate              `json:"gate"`
	Lifecycle    []Event           `json:"lifecycle"`
	Steps        []Check           `json:"steps"`
	Checks       []Check           `json:"checks"`
	Proofs       []json.RawMessage `json:"proofs"`
	Observations []Observation     `json:"observations"`
	TargetCount  *int64            `json:"targetCount"`
	Decision     string            `json:"decision"`
	Diagnostics  []Diagnostic      `json:"diagnostics"`
	Artifacts    []json.RawMessage `json:"artifacts"`
}

var hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validateReport(b []byte, h Harness, root, digest string, exit *int) (Report, error) {
	var r Report
	bad := func() (Report, error) { return r, config.Err("OUTPUT_INVALID", "invalid Invaris check evidence") }
	if len(b) == 0 {
		return r, config.Err("OUTPUT_MISSING", "Invaris emitted no evidence")
	}
	if config.DecodeStrict(b, &r) != nil {
		return bad()
	}
	if r.SchemaVersion != EvidenceSchema {
		return r, config.Err("UNSUPPORTED_SCHEMA", "Invaris evidence version")
	}
	if r.ConfigDigest == nil || *r.ConfigDigest != digest {
		return r, config.Err("EVIDENCE_MISMATCH", "Invaris loaded different harness bytes")
	}
	if !config.IDPattern.MatchString(r.RunID) || r.Action != "check" || r.Target != root || r.Harness != h.Name || r.Compatibility.Status != "native" || r.Compatibility.InputVersion != HarnessSchema || !reflect.DeepEqual(r.Policies, h.Policies) || !reflect.DeepEqual(r.Gate, h.Gate) || len(r.Steps)+len(r.Proofs)+len(r.Artifacts) != 0 || len(r.Checks) != len(h.Guards) || len(r.Lifecycle) != len(h.Guards) || r.StartedAt.IsZero() || r.FinishedAt.Before(r.StartedAt) || r.DurationMs < 0 || r.Environment.OS == "" || r.Environment.Arch == "" {
		return bad()
	}
	if r.GitRevision != nil && !config.RevisionPattern.MatchString(*r.GitRevision) || r.TargetCount != nil && *r.TargetCount < 0 {
		return bad()
	}
	failed, missing := false, false
	for i, g := range h.Guards {
		c, e := r.Checks[i], r.Lifecycle[i]
		if c.ID != g.ID || c.PolicyID != g.PolicyID || c.ProviderID != "invaris-command" || e.ID != g.ID || e.Kind != "guard" || e.Status != c.Status || c.Inspected != nil && *c.Inspected < 0 {
			return bad()
		}
		if e.StartedAt != nil && e.FinishedAt != nil && e.FinishedAt.Before(*e.StartedAt) {
			return bad()
		}
		switch c.Status {
		case "PASS":
			if c.Inspected == nil || *c.Inspected < 1 || *c.Inspected < int64(g.MinInspected) || c.ExitCode == nil || *c.ExitCode != 0 || len(c.Findings) != 0 {
				return bad()
			}
		case "FAIL":
			failed = true
			if c.ExitCode == nil || *c.ExitCode != 1 || len(c.Findings) == 0 || c.Inspected == nil || *c.Inspected < 1 || *c.Inspected < int64(g.MinInspected) {
				return bad()
			}
		case "REFUSE", "NOT_RUN":
			missing = true
		default:
			return bad()
		}
		for _, f := range c.Findings {
			if f.PolicyID != g.PolicyID || f.RuleID == "" || f.Path == "" || f.Line != nil && *f.Line < 1 {
				return bad()
			}
		}
		for _, d := range c.Diagnostics {
			if d.Code == "" {
				return bad()
			}
		}
	}
	seen := map[string]bool{}
	for _, o := range r.Observations {
		if o.Kind != "guard" || seen[o.ID] || o.Provider.Name != "invaris-command" {
			return bad()
		}
		seen[o.ID] = true
		found := false
		for _, g := range h.Guards {
			if o.ID == g.ID && o.Program == g.Program {
				found = true
			}
		}
		if !found {
			return bad()
		}
		if o.StartedAt != nil && o.FinishedAt != nil && o.FinishedAt.Before(*o.StartedAt) {
			return bad()
		}
		if o.Executable != nil && (o.Executable.Path == "" || o.Executable.SHA256 != nil && !hex.MatchString(*o.Executable.SHA256)) {
			return bad()
		}
		if o.TimeoutSeconds != nil && *o.TimeoutSeconds < 1 {
			return bad()
		}
	}
	for _, c := range r.Checks {
		if c.Status != "NOT_RUN" && !seen[c.ID] {
			return bad()
		}
	}
	want, code := "ACCEPT", 0
	if failed {
		want, code = "REJECT", 1
	} else if missing {
		want, code = "REFUSE", 2
	}
	if r.Decision != want {
		return bad()
	}
	if r.Decision == "ACCEPT" && (r.TargetCount == nil || *r.TargetCount < 1) {
		return bad()
	}
	if exit == nil || *exit != code {
		return r, config.Err("EXIT_MISMATCH", "Invaris exit/report mismatch")
	}
	return r, nil
}

// Invaris 0.5 has no diagnostic severity. For the supported command-check
// profile, diagnostics block PASS. A structured rejection may carry
// COMMAND_FAILED from the expected exit 1. Compatibility warnings and
// bounded excerpts are separate from evidence completeness.
func (r Report) incompleteEvidence() bool {
	if len(r.Diagnostics) != 0 {
		return true
	}
	failed := map[string]bool{}
	for _, c := range r.Checks {
		failed[c.ID] = c.Status == "FAIL"
		if blockingDiagnostics(c.Diagnostics, failed[c.ID]) {
			return true
		}
	}
	for _, o := range r.Observations {
		if blockingDiagnostics(o.Diagnostics, failed[o.ID]) {
			return true
		}
	}
	return false
}

func blockingDiagnostics(diagnostics []Diagnostic, knownFailure bool) bool {
	for _, d := range diagnostics {
		if !knownFailure || d.Code != "COMMAND_FAILED" {
			return true
		}
	}
	return false
}
