package stash_test

import (
	"strings"
	"testing"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/stash"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func TestStepKeys(t *testing.T) {
	tests := []struct {
		name string
		key  func(string) (string, error)
		want string
	}{
		{"step", stash.KeyStepV1, "md/v1/step/src"},
		{"changes", stash.KeyChangesV1, "md/v1/changes/src"},
		{"compliance", stash.KeyComplianceV1, "md/v1/compliance/src"},
		{"policy", stash.KeyPolicyV1, "md/v1/policy/src"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.key("src")
			if err != nil || got != tc.want {
				t.Errorf("got (%q, %v), want %q", got, err, tc.want)
			}
		})
	}
}

func TestStepKeys_Segment(t *testing.T) {
	tests := []struct {
		step string
		ok   bool
	}{
		{"src", true},
		{"0", true},
		{"helm-chart_v2", true},
		{strings.Repeat("a", 63), true},
		{"", false},
		{strings.Repeat("a", 64), false},
		{"Src", false},
		{"-src", false},
		{"_src", false},
		{"src/app", false},
		{"src.app", false},
	}
	for _, tc := range tests {
		t.Run(tc.step, func(t *testing.T) {
			_, err := stash.KeyStepV1(tc.step)
			if (err == nil) != tc.ok {
				t.Errorf("KeyStepV1(%q) err = %v, want ok = %v", tc.step, err, tc.ok)
			}
		})
	}
}

func TestJSONInput_Results(t *testing.T) {
	at := time.Date(2026, 11, 27, 20, 3, 24, 0, time.UTC)
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			"step",
			stash.Step{Tool: "opentofu", ToolVersion: "1.10.6", Action: stash.StepApply, Status: stash.StepSucceeded, StartedAt: at, DurationS: types.Ptr(142.0), ExitCode: types.Ptr(0), Message: "Apply complete"},
			`{"tool":"opentofu","tool_version":"1.10.6","action":"apply","status":"succeeded","started_at":"2026-11-27T20:03:24Z","duration_s":142,"exit_code":0,"message":"Apply complete"}`,
		},
		{
			"step with required fields only",
			stash.Step{Tool: "helm", Action: stash.StepPlan, Status: stash.StepRunning},
			`{"tool":"helm","action":"plan","status":"running"}`,
		},
		{
			"changes",
			stash.Changes{Tool: "opentofu", Add: 2, Change: 3, Destroy: 0, Replace: types.Ptr(1), NoOp: types.Ptr(41), Resources: []stash.ChangedResource{{Address: "aws_ecs_service.app", Action: stash.ChangeUpdate}}},
			`{"tool":"opentofu","add":2,"change":3,"destroy":0,"replace":1,"no_op":41,"resources":[{"address":"aws_ecs_service.app","action":"update"}]}`,
		},
		{
			"compliance",
			stash.Compliance{Tool: "checkov", Passed: 36, Failed: 2, Skipped: types.Ptr(4), Severity: &stash.Severity{High: 1, Medium: 1}, Blocking: types.Ptr(false), ReportURL: "https://ci.example.com/r/1"},
			`{"tool":"checkov","passed":36,"failed":2,"skipped":4,"severity":{"critical":0,"high":1,"medium":1,"low":0},"blocking":false,"report_url":"https://ci.example.com/r/1"}`,
		},
		{
			"policy",
			stash.Policy{Tool: "conftest", Passed: 12, Failed: 1, Warnings: types.Ptr(2), Denials: []string{"Production databases must be Multi-AZ"}},
			`{"tool":"conftest","passed":12,"failed":1,"warnings":2,"denials":["Production databases must be Multi-AZ"]}`,
		},
		{
			"vulnerabilities",
			stash.Vulnerabilities{Tool: "trivy", Target: "visits:a41c9e2", Fixable: types.Ptr(0)},
			`{"tool":"trivy","target":"visits:a41c9e2","severity":{"critical":0,"high":0,"medium":0,"low":0},"fixable":0}`,
		},
		{
			"tests",
			stash.Tests{Tool: "smoke", Passed: 8, Failed: 0, Skipped: types.Ptr(1), DurationS: types.Ptr(21.0)},
			`{"tool":"smoke","passed":8,"failed":0,"skipped":1,"duration_s":21}`,
		},
		{
			"cost estimate",
			stash.CostEstimate{Tool: "infracost", Currency: "USD", MonthlyBefore: types.Ptr(41.2), MonthlyAfter: 41.58, Delta: types.Ptr(0.38)},
			`{"tool":"infracost","currency":"USD","monthly_before":41.2,"monthly_after":41.58,"delta":0.38}`,
		},
		{
			"source",
			stash.Source{Repo: "tendwell-health/visits", Ref: "main", Commit: "a41c9e2"},
			`{"repo":"tendwell-health/visits","ref":"main","commit":"a41c9e2"}`,
		},
		{
			"state lock",
			stash.StateLock{Tool: "opentofu", Status: stash.StateLockReleased, Holder: "deployment"},
			`{"tool":"opentofu","status":"released","holder":"deployment"}`,
		},
		{
			"drift",
			stash.Drift{Status: stash.DriftNone, Resources: types.Ptr(0), CheckedAt: at},
			`{"status":"none","resources":0,"checked_at":"2026-11-27T20:03:24Z"}`,
		},
		{
			"release",
			stash.Release{Version: "2026.11.27-5", Image: "visits:a41c9e2", DeployedAt: at},
			`{"version":"2026.11.27-5","image":"visits:a41c9e2","deployed_at":"2026-11-27T20:03:24Z"}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stash.JSONInput(tc.value)
			if err != nil {
				t.Fatalf("JSONInput: %v", err)
			}
			if got.Value != tc.want || got.Format != stash.FormatJSON {
				t.Errorf("got %s, want %s", got.Value, tc.want)
			}
		})
	}
}
