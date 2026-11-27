package stash

import (
	"fmt"
	"regexp"
	"time"
)

// Deployment result keys. A provisioner sets them on its deployment, except
// [KeyDriftV1] and [KeyReleaseV1], which go on the instance. An instance's
// widgets include the results of its latest deployment that has any.
//
// The per-step keys take the step name as a segment; build them with
// [KeyStepV1], [KeyChangesV1], [KeyComplianceV1], and [KeyPolicyV1].
const (
	// KeyVulnerabilitiesV1 is a JSON key on a deployment. Its value is a
	// [Vulnerabilities].
	KeyVulnerabilitiesV1 = "md/v1/vulnerabilities"
	// KeyTestsV1 is a JSON key on a deployment. Its value is a [Tests].
	KeyTestsV1 = "md/v1/tests"
	// KeyCostEstimateV1 is a JSON key on a deployment. Its value is a
	// [CostEstimate].
	KeyCostEstimateV1 = "md/v1/cost_estimate"
	// KeySourceV1 is a JSON key on a deployment. Its value is a [Source].
	KeySourceV1 = "md/v1/source"
	// KeyStateLockV1 is a JSON key on a deployment. Its value is a
	// [StateLock].
	KeyStateLockV1 = "md/v1/state_lock"
	// KeyDriftV1 is a JSON key on an instance. Its value is a [Drift].
	KeyDriftV1 = "md/v1/drift"
	// KeyReleaseV1 is a JSON key on an instance. Its value is a [Release].
	KeyReleaseV1 = "md/v1/release"
)

// stepPattern is the server's rule for a key segment: a bundle step path of
// 1 to 63 lowercase letters, digits, `_`, and `-` that starts with a letter
// or digit.
var stepPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// KeyStepV1 returns `md/v1/step/<step>`, whose value is a [Step].
func KeyStepV1(step string) (string, error) { return stepKey("step", step) }

// KeyChangesV1 returns `md/v1/changes/<step>`, whose value is a [Changes].
func KeyChangesV1(step string) (string, error) { return stepKey("changes", step) }

// KeyComplianceV1 returns `md/v1/compliance/<step>`, whose value is a
// [Compliance].
func KeyComplianceV1(step string) (string, error) { return stepKey("compliance", step) }

// KeyPolicyV1 returns `md/v1/policy/<step>`, whose value is a [Policy].
func KeyPolicyV1(step string) (string, error) { return stepKey("policy", step) }

func stepKey(name, step string) (string, error) {
	if !stepPattern.MatchString(step) {
		return "", fmt.Errorf("invalid step %q: use 1 to 63 lowercase letters, digits, _ and -, starting with a letter or digit", step)
	}
	return "md/v1/" + name + "/" + step, nil
}

// Severity counts findings by severity. All four counts are sent.
type Severity struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// StepAction is what a step did.
type StepAction string

const (
	StepPlan    StepAction = "plan"
	StepApply   StepAction = "apply"
	StepDestroy StepAction = "destroy"
	StepTest    StepAction = "test"
	StepScan    StepAction = "scan"
)

// StepStatus is how a step ended, or [StepRunning] while it runs.
type StepStatus string

const (
	StepRunning   StepStatus = "running"
	StepSucceeded StepStatus = "succeeded"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
	StepCanceled  StepStatus = "canceled"
)

// Step is the value of [KeyStepV1]: how a step ran. Tool, Action, and
// Status are required. Optional fields are left out when empty or nil.
type Step struct {
	// Tool is up to 64 characters, for example "opentofu".
	Tool        string     `json:"tool"`
	ToolVersion string     `json:"tool_version,omitempty"`
	Action      StepAction `json:"action"`
	Status      StepStatus `json:"status"`
	StartedAt   time.Time  `json:"started_at,omitzero"`
	FinishedAt  time.Time  `json:"finished_at,omitzero"`
	DurationS   *float64   `json:"duration_s,omitempty"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	// Message is up to 280 characters.
	Message string `json:"message,omitempty"`
}

// ChangeAction is what a plan or apply does to one resource.
type ChangeAction string

const (
	ChangeCreate  ChangeAction = "create"
	ChangeUpdate  ChangeAction = "update"
	ChangeDelete  ChangeAction = "delete"
	ChangeReplace ChangeAction = "replace"
)

// Changes is the value of [KeyChangesV1]: what a plan or apply changes.
// Tool, Add, Change, and Destroy are required.
type Changes struct {
	Tool        string `json:"tool"`
	ToolVersion string `json:"tool_version,omitempty"`
	Add         int    `json:"add"`
	Change      int    `json:"change"`
	Destroy     int    `json:"destroy"`
	Replace     *int   `json:"replace,omitempty"`
	NoOp        *int   `json:"no_op,omitempty"`
	// Resources holds up to 50 changed resources.
	Resources []ChangedResource `json:"resources,omitempty"`
}

// ChangedResource is one resource in [Changes]. Address is up to 512
// characters.
type ChangedResource struct {
	Address string       `json:"address"`
	Action  ChangeAction `json:"action"`
}

// Compliance is the value of [KeyComplianceV1]: findings from an IaC or
// config scanner. Tool, Passed, and Failed are required.
type Compliance struct {
	Tool        string    `json:"tool"`
	ToolVersion string    `json:"tool_version,omitempty"`
	Passed      int       `json:"passed"`
	Failed      int       `json:"failed"`
	Skipped     *int      `json:"skipped,omitempty"`
	Severity    *Severity `json:"severity,omitempty"`
	// Blocking is true when the findings stop the deployment.
	Blocking  *bool  `json:"blocking,omitempty"`
	ReportURL string `json:"report_url,omitempty"`
}

// Policy is the value of [KeyPolicyV1]: the results of a policy engine.
// Tool, Passed, and Failed are required.
type Policy struct {
	Tool        string `json:"tool"`
	ToolVersion string `json:"tool_version,omitempty"`
	Passed      int    `json:"passed"`
	Failed      int    `json:"failed"`
	Warnings    *int   `json:"warnings,omitempty"`
	// Denials holds up to 50 messages of up to 280 characters each.
	Denials []string `json:"denials,omitempty"`
}

// Vulnerabilities is the value of [KeyVulnerabilitiesV1]: findings from an
// image or package scan. Tool and Severity are required.
type Vulnerabilities struct {
	Tool        string `json:"tool"`
	ToolVersion string `json:"tool_version,omitempty"`
	// Target is what was scanned, such as an image, up to 512 characters.
	Target   string   `json:"target,omitempty"`
	Severity Severity `json:"severity"`
	Fixable  *int     `json:"fixable,omitempty"`
}

// Tests is the value of [KeyTestsV1]: test results. Tool, Passed, and
// Failed are required.
type Tests struct {
	Tool        string   `json:"tool"`
	ToolVersion string   `json:"tool_version,omitempty"`
	Passed      int      `json:"passed"`
	Failed      int      `json:"failed"`
	Skipped     *int     `json:"skipped,omitempty"`
	DurationS   *float64 `json:"duration_s,omitempty"`
}

// CostEstimate is the value of [KeyCostEstimateV1]: the monthly cost before
// and after a change. Tool, Currency, and MonthlyAfter are required.
type CostEstimate struct {
	Tool        string `json:"tool"`
	ToolVersion string `json:"tool_version,omitempty"`
	// Currency is a three-letter code, for example "USD".
	Currency      string   `json:"currency"`
	MonthlyBefore *float64 `json:"monthly_before,omitempty"`
	MonthlyAfter  float64  `json:"monthly_after"`
	Delta         *float64 `json:"delta,omitempty"`
}

// Source is the value of [KeySourceV1]: which code was deployed. Commit is
// required, up to 64 characters.
type Source struct {
	Repo    string `json:"repo,omitempty"`
	Ref     string `json:"ref,omitempty"`
	Commit  string `json:"commit"`
	Author  string `json:"author,omitempty"`
	Message string `json:"message,omitempty"`
	URL     string `json:"url,omitempty"`
}

// StateLockStatus is the state of a state lock.
type StateLockStatus string

const (
	StateLockAcquired      StateLockStatus = "acquired"
	StateLockReleased      StateLockStatus = "released"
	StateLockForceUnlocked StateLockStatus = "force_unlocked"
)

// StateLock is the value of [KeyStateLockV1]: the state lock during a
// step. Tool and Status are required.
type StateLock struct {
	Tool        string          `json:"tool"`
	ToolVersion string          `json:"tool_version,omitempty"`
	Status      StateLockStatus `json:"status"`
	Holder      string          `json:"holder,omitempty"`
}

// DriftStatus is whether real infrastructure matches the config.
type DriftStatus string

const (
	DriftNone     DriftStatus = "none"
	DriftDetected DriftStatus = "detected"
	DriftUnknown  DriftStatus = "unknown"
)

// Drift is the value of [KeyDriftV1]. Status is required.
type Drift struct {
	Status DriftStatus `json:"status"`
	// Resources is the number of drifted resources.
	Resources *int      `json:"resources,omitempty"`
	CheckedAt time.Time `json:"checked_at,omitzero"`
}

// Release is the value of [KeyReleaseV1]: what is running now. Version is
// required, up to 128 characters.
type Release struct {
	Version    string    `json:"version"`
	Image      string    `json:"image,omitempty"`
	DeployedAt time.Time `json:"deployed_at,omitzero"`
}
