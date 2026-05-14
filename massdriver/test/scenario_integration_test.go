//go:build integration

package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/components"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/environments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/instances"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
)

// scenarioOCIRepoName is the OCI repo the scenario sources its
// component from. The sandbox must have a published bundle there.
const scenarioOCIRepoName = "sdk-integration-test"

// Fixed alphanumeric IDs for the scenario fixtures (project IDs are
// capped at 20 chars; identifiers within a project must be lowercase
// alphanumeric).
const (
	scenarioProjectID = "inttestscn"
	scenarioEnvID     = "scenarioenv"
	scenarioCompID    = "scenariocomp"
)

// TestIntegration_Scenario_ProjectEnvComponentInstance walks the
// project → environment → component → instance materialization.
// Does not trigger a deployment.
func TestIntegration_Scenario_ProjectEnvComponentInstance(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, _ = c.Projects.Delete(ctx, scenarioProjectID) // best-effort pre-clean

	if _, err := c.Projects.Create(ctx, projects.CreateInput{
		ID:   scenarioProjectID,
		Name: "Scenario test",
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Projects.Delete(ctx, scenarioProjectID)
	})

	envCreated, err := c.Environments.Create(ctx, scenarioProjectID, environments.CreateInput{
		ID:   scenarioEnvID,
		Name: "Scenario env",
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	compCreated, err := c.Components.Add(ctx, scenarioProjectID, components.AddInput{
		OciRepoName: scenarioOCIRepoName,
		ID:          scenarioCompID,
		Name:        "Scenario component",
	})
	if err != nil {
		t.Fatalf("add component (sandbox missing %q OCI repo?): %v", scenarioOCIRepoName, err)
	}

	inst, err := waitForInstance(ctx, c, envCreated.ID, compCreated.ID, 30*time.Second)
	if err != nil {
		t.Fatalf("instance did not materialize: %v", err)
	}

	if inst.Component == nil || inst.Component.ID != compCreated.ID {
		t.Errorf("instance.Component = %+v, want id %s", inst.Component, compCreated.ID)
	}
	if inst.Environment == nil || inst.Environment.ID != envCreated.ID {
		t.Errorf("instance.Environment = %+v, want id %s", inst.Environment, envCreated.ID)
	}
	if inst.Status == "" {
		t.Errorf("instance.Status is empty; expected a populated value")
	}
}

// waitForInstance polls Instances.List until an instance produced by
// (envID, compID) appears, or timeout elapses.
func waitForInstance(ctx context.Context, c *massdriver.Client, envID, compID string, timeout time.Duration) (*instances.Instance, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		list, err := c.Instances.List(ctx, instances.ListInput{
			EnvironmentID: envID,
		})
		if err != nil {
			return nil, fmt.Errorf("list instances: %w", err)
		}
		for i := range list {
			if list[i].Component != nil && list[i].Component.ID == compID {
				return &list[i], nil
			}
		}
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("no instance found in env %s for component %s after %s", envID, compID, timeout)
}
