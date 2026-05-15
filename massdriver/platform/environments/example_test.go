package environments_test

import (
	"context"
	"fmt"
	"log"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// StreamEvents subscribes to an environment's event feed: updates and
// deletes for the environment itself, lifecycle events for every
// instance / connection / alarm in it, deployments run against those
// instances, and the environment's default-resource assignments.
//
// Environment creation events are delivered on the parent project's
// subscription (c.Projects.StreamEvents), so listen to both for full
// lifecycle coverage.
//
// Cancel ctx to tear down the subscription and close the channel.
func ExampleService_StreamEvents() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := c.Environments.StreamEvents(ctx, "ecommerce-prod")
	if err != nil {
		log.Fatal(err)
	}
	for ev := range events {
		switch e := ev.(type) {
		case *types.EnvironmentEvent:
			fmt.Printf("%s: environment %s\n", e.Action, e.Environment.Name)
		case *types.EnvironmentDefaultEvent:
			fmt.Printf("%s: default %s\n", e.Action, e.EnvironmentDefault.Resource.Name)
		case *types.InstanceEvent:
			fmt.Printf("%s: instance %s (%s)\n", e.Action, e.Instance.Name, e.Instance.Status)
		case *types.ConnectionEvent:
			fmt.Printf("%s: connection %s → %s\n", e.Action, e.Connection.FromField, e.Connection.ToField)
		case *types.AlarmEvent:
			fmt.Printf("%s: alarm %s\n", e.Action, e.Alarm.DisplayName)
		case *types.DeploymentEvent:
			fmt.Printf("%s: deployment %s is %s\n", e.Action, e.Deployment.ID, e.Deployment.Status)
		}
	}
}
