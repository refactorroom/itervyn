package evaris

// Wire contracts follow the Evaris release lines in process.SupportedReleases
// (0.4.x and 0.5.x, Apache-2.0). Grading and store algorithms remain upstream.
import (
	"github.com/akkaraponph/itervyn/internal/config"
	"time"
)

const EvaluationSchema = "evaris.evaluation/v1alpha1"

type FileRef struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Bytes      int64  `json:"bytes"`
	Executable bool   `json:"executable"`
}
type Finding struct {
	PolicyID string `json:"policyId"`
	RuleID   string `json:"ruleId"`
	Path     string `json:"path"`
	// Line is the 1-based line Invaris reported (Evaris 0.4+).
	Line    *int   `json:"line" optional:"true"`
	Message string `json:"message"`
}
type Verification struct {
	Required      bool      `json:"required"`
	Status        string    `json:"status"`
	SchemaVersion *string   `json:"schemaVersion"`
	ExitCode      *int      `json:"exitCode"`
	RunID         *string   `json:"runId"`
	ConfigDigest  *string   `json:"configDigest"`
	Executable    *FileRef  `json:"executable"`
	Evidence      []FileRef `json:"evidence"`
	Reason        string    `json:"reason"`
	// Message is a catalogue message naming the failing step (Evaris 0.4+).
	Message  string    `json:"message" optional:"true"`
	Findings []Finding `json:"findings"`
}
type Grader struct {
	CriterionID   string    `json:"criterionId"`
	Required      bool      `json:"required"`
	Status        string    `json:"status"`
	Reason        string    `json:"reason"`
	Message       string    `json:"message"`
	Evidence      []FileRef `json:"evidence"`
	GraderVersion string    `json:"graderVersion"`
}
type Evaluation struct {
	SchemaVersion string        `json:"schemaVersion"`
	ID            string        `json:"id"`
	RunID         string        `json:"runId"`
	PreparationID string        `json:"preparationId"`
	ExperimentID  string        `json:"experimentId"`
	CaseID        string        `json:"caseId"`
	ArmID         string        `json:"armId"`
	RunDigest     string        `json:"runDigest"`
	OracleDigest  string        `json:"oracleDigest"`
	CreatedAt     time.Time     `json:"createdAt"`
	Execution     string        `json:"execution"`
	AgentOutcome  string        `json:"agentOutcome"`
	Graders       []Grader      `json:"graders"`
	Review        string        `json:"review"`
	Verification  *Verification `json:"verification"`
	Assessment    string        `json:"assessment"`
	Limitations   []string      `json:"limitations"`
}

func one(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
func parse(b []byte, run string, exit *int) (Evaluation, error) {
	var e Evaluation
	bad := func() (Evaluation, error) {
		return e, config.Err("OUTPUT_INVALID", "invalid Evaris evaluation evidence")
	}
	if len(b) == 0 {
		return e, config.Err("OUTPUT_MISSING", "Evaris emitted no evaluation")
	}
	if config.DecodeStrict(b, &e) != nil {
		return bad()
	}
	if e.SchemaVersion != EvaluationSchema {
		return e, config.Err("UNSUPPORTED_SCHEMA", "Evaris evaluation version")
	}
	if !config.IDPattern.MatchString(e.ID) || !config.IDPattern.MatchString(e.PreparationID) || e.RunID != run || !config.IDPattern.MatchString(e.RunID) || e.ExperimentID == "" || e.CaseID == "" || e.ArmID == "" || !config.DigestPattern.MatchString(e.RunDigest) || !config.DigestPattern.MatchString(e.OracleDigest) || e.CreatedAt.IsZero() || !one(e.Execution, "COMPLETE", "INCOMPLETE", "ERROR", "CANCELED") || !one(e.AgentOutcome, "SATISFIED", "VIOLATED", "UNKNOWN") || !one(e.Review, "NOT_REQUIRED", "PENDING") || !one(e.Assessment, "PASS", "FAIL", "INCONCLUSIVE") {
		return bad()
	}
	seen := map[string]bool{}
	required := 0
	for _, g := range e.Graders {
		if g.CriterionID == "" || seen[g.CriterionID] || g.GraderVersion != "builtin/1" || !one(g.Status, "PASS", "FAIL", "UNAVAILABLE", "ERROR") {
			return bad()
		}
		seen[g.CriterionID] = true
		if g.Required {
			required++
		}
		for _, r := range g.Evidence {
			if !config.SafeRelative(r.Path) || !config.DigestPattern.MatchString(r.SHA256) || r.Bytes < 0 {
				return bad()
			}
		}
	}
	if required == 0 {
		return bad()
	}
	if v := e.Verification; v != nil {
		if !one(v.Status, "NOT_CONFIGURED", "ACCEPT", "REJECT", "REFUSE", "ERROR", "UNAVAILABLE") {
			return bad()
		}
		if v.RunID != nil && !config.IDPattern.MatchString(*v.RunID) || v.ConfigDigest != nil && !config.DigestPattern.MatchString(*v.ConfigDigest) {
			return bad()
		}
		for _, r := range v.Evidence {
			if !config.SafeRelative(r.Path) || !config.DigestPattern.MatchString(r.SHA256) || r.Bytes < 0 {
				return bad()
			}
		}
		for _, f := range v.Findings {
			if f.Line != nil && *f.Line < 1 {
				return bad()
			}
		}
	}
	code := 0
	if e.Assessment == "FAIL" {
		code = 1
	} else if e.Assessment == "INCONCLUSIVE" {
		code = 2
	}
	if exit == nil || *exit != code {
		return e, config.Err("EXIT_MISMATCH", "Evaris exit/evaluation mismatch")
	}
	return e, nil
}

// Evaris can assess PASS while reporting unavailable optional criteria or
// optional verification. Those reasons and import limitations stay advisory.
// Missing required checks, pending review, and cancellation are blocking.
func (e Evaluation) incompleteEvidence() bool {
	if e.Review != "NOT_REQUIRED" || e.Execution == "CANCELED" || e.Assessment == "PASS" && e.AgentOutcome != "SATISFIED" {
		return true
	}
	for _, g := range e.Graders {
		if g.Required && g.Status != "PASS" && (g.Status != "FAIL" || e.Assessment == "PASS") {
			return true
		}
	}
	return e.Verification != nil && e.Verification.Required &&
		e.Verification.Status != "ACCEPT" && (e.Verification.Status != "REJECT" || e.Assessment == "PASS")
}
