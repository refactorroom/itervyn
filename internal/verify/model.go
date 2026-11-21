// Package verify owns verification policy and the versioned Itervyn result.
package verify

import "time"

const (
	Version           = "v0.1.0"
	ResultSchema      = "itervyn.result/v1alpha1"
	GatedResultSchema = "itervyn.result/v1alpha2"
	DiagnosticSchema  = "itervyn.diagnostic/v1alpha1"
	DoctorSchema      = "itervyn.doctor/v1alpha1"
)

type Status string

const (
	Pass         Status = "PASS"
	Fail         Status = "FAIL"
	Inconclusive Status = "INCONCLUSIVE"
	Error        Status = "ERROR"
)

type NextAction string

const (
	Complete    NextAction = "COMPLETE"
	Fix         NextAction = "FIX_AND_REVERIFY"
	Investigate NextAction = "INVESTIGATE"
	Stop        NextAction = "STOP"
)

type ArtifactRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type Feedback struct {
	Provider     *string `json:"provider"`
	Code         string  `json:"code"`
	Message      string  `json:"message"`
	EvidencePath *string `json:"evidencePath"`
}
type Scope struct {
	Kind          string  `json:"kind"`
	ProjectRoot   *string `json:"projectRoot"`
	Store         *string `json:"store"`
	ExperimentID  *string `json:"experimentId"`
	PreparationID *string `json:"preparationId"`
	RunID         *string `json:"runId"`
	EvaluationID  *string `json:"evaluationId"`
	CaseID        *string `json:"caseId"`
	ArmID         *string `json:"armId"`
	RunDigest     *string `json:"runDigest"`
	OracleDigest  *string `json:"oracleDigest"`
}
type ProviderResult struct {
	Name                string        `json:"name"`
	Required            bool          `json:"required"`
	Status              Status        `json:"status"`
	NativeDecision      *string       `json:"nativeDecision"`
	NativeSchemaVersion *string       `json:"nativeSchemaVersion"`
	ExitCode            *int          `json:"exitCode"`
	Reason              *string       `json:"reason"`
	NativeReasons       []string      `json:"nativeReasons"`
	StartedAt           *time.Time    `json:"startedAt"`
	FinishedAt          *time.Time    `json:"finishedAt"`
	ExecutableSHA256    *string       `json:"executableSha256"`
	Scope               Scope         `json:"scope"`
	Evidence            []ArtifactRef `json:"evidence"`
}
type Result struct {
	SchemaVersion     string           `json:"schemaVersion"`
	RunID             string           `json:"runId"`
	TaskID            *string          `json:"taskId"`
	Status            Status           `json:"status"`
	NextAction        NextAction       `json:"nextAction"`
	Providers         []ProviderResult `json:"providers"`
	Feedback          []Feedback       `json:"feedback"`
	Warnings          []Feedback       `json:"warnings"`
	StartedAt         time.Time        `json:"startedAt"`
	FinishedAt        time.Time        `json:"finishedAt"`
	ConfigSHA256      *string          `json:"configSha256"`
	ProjectRoot       *string          `json:"projectRoot"`
	GitRevision       *string          `json:"gitRevision"`
	GitDirty          *bool            `json:"gitDirty"`
	RevisionGate      *RevisionGate    `json:"revisionGate,omitempty" optional:"true"`
	Artifacts         []ArtifactRef    `json:"artifacts"`
	EvidencePersisted bool             `json:"evidencePersisted"`
}
type Diagnostic struct {
	SchemaVersion     string  `json:"schemaVersion"`
	Code              string  `json:"code"`
	Message           string  `json:"message"`
	RunID             *string `json:"runId"`
	Status            Status  `json:"status"`
	EvidencePersisted bool    `json:"evidencePersisted"`
}

func Ptr[T any](v T) *T { return &v }
func Exit(s Status) int {
	if s == Pass {
		return 0
	}
	if s == Fail {
		return 1
	}
	return 2
}
