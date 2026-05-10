//go:build integration

package massdriver_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/components"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/environments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/instances"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
)

// scenarioOCIRepoName is the OCI repo the scenario test sources its
// component from. The sandbox must have a published bundle in this
// repo for the test to pass — convention is to publish a minimal
// bundle here specifically for SDK integration testing.
const scenarioOCIRepoName = "sdk-integration-test"

// TestIntegration_Scenario_ProjectEnvComponentInstance exercises the
// load-bearing cross-package shape: project → environment → component
// → instance.
//
// When a component is added to a project blueprint and the project has
// at least one environment, the platform materializes an instance.
// This test does NOT deploy anything — it only verifies that the
// instance appears with the expected linkage. A full deployment test
// (which would consume real cloud resources) is deferred; run
// deployment scenarios manually against a sandbox bundle that's
// known to be safe to provision.
//
// Cleanup tears down in reverse-creation order via t.Cleanup's LIFO
// stack: component → environment → project.
func TestIntegration_Scenario_ProjectEnvComponentInstance(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	// 1. Create project.
	projID := inttest.FixtureName(t, "scenario-proj")
	if _, err := c.Projects.Create(ctx, projects.CreateInput{
		ID:          projID,
		Name:        "Scenario test " + projID,
		Description: "Cross-package SDK scenario test; safe to delete.",
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.Projects.Delete(ctx, projID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup project %s: %v", projID, err)
		}
	})

	// 2. Create environment in project.
	envID := inttest.FixtureName(t, "scenario-env")
	if _, err := c.Environments.Create(ctx, projID, environments.CreateInput{
		ID:   envID,
		Name: "Scenario env " + envID,
	}); err != nil {
		t.Fatalf("create environment: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.Environments.Delete(ctx, envID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup environment %s: %v", envID, err)
		}
	})

	// 3. Add component to project blueprint, sourcing from the
	// sandbox OCI repo.
	compID := inttest.FixtureName(t, "scenario-comp")
	if _, err := c.Components.Add(ctx, projID, scenarioOCIRepoName, components.AddInput{
		ID:   compID,
		Name: "Scenario component",
	}); err != nil {
		t.Fatalf("add component (sandbox missing %q OCI repo?): %v", scenarioOCIRepoName, err)
	}
	t.Cleanup(func() {
		if _, err := c.Components.Remove(ctx, compID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup component %s: %v", compID, err)
		}
	})

	// 4. Wait for the platform to materialize an instance for the new
	// component in the new environment. The platform creates this
	// asynchronously; allow some time before failing.
	inst, err := waitForInstance(ctx, c, envID, compID, 30*time.Second)
	if err != nil {
		t.Fatalf("instance did not materialize: %v", err)
	}

	// Verify the instance carries back-references to the project,
	// environment, and component we created.
	if inst.Component == nil || inst.Component.ID != compID {
		t.Errorf("instance.Component = %+v, want id %s", inst.Component, compID)
	}
	if inst.Environment == nil || inst.Environment.ID != envID {
		t.Errorf("instance.Environment = %+v, want id %s", inst.Environment, envID)
	}

	// Status should be initialized (no deploy yet) — the value comes
	// straight from the platform; just assert it's set.
	if inst.Status == "" {
		t.Errorf("instance.Status is empty; expected at least an INITIALIZED-shaped value")
	}
}

// waitForInstance polls Instances.List until an instance produced by
// (envID, compID) appears, or timeout elapses. Returns the instance
// or an error explaining why the wait failed.
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
