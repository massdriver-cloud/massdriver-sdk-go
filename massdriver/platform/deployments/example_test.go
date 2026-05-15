package deployments_test

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/deployments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func ExampleService_Create() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	dep, err := c.Deployments.Create(context.Background(), "ecomm-prod-database", deployments.CreateInput{
		Action: deployments.ActionProvision,
		Params: map[string]any{"size": "small", "version": "14"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("queued deployment %s (%s)\n", dep.ID, dep.Status)
}

// TailLogs writes the deployment's full log stream — backfill plus
// every batch arriving over the live subscription — to the supplied
// writer. The call returns when the deployment reaches a terminal
// status, ctx is cancelled, or the writer returns an error.
func ExampleService_TailLogs() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	if err := c.Deployments.TailLogs(context.Background(), "deploy-abc123", os.Stdout); err != nil {
		log.Fatal(err)
	}
}

// StreamEvents subscribes to a single deployment's lifecycle
// transitions — fires on create and every status flip (PENDING →
// RUNNING → COMPLETED). Useful when you want to react to a deployment
// reaching a terminal state without polling. For log content, use
// TailLogs / StreamLogs instead.
func ExampleService_StreamEvents() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := c.Deployments.StreamEvents(ctx, "deploy-abc123")
	if err != nil {
		log.Fatal(err)
	}
	for ev := range events {
		e := ev.(*types.DeploymentEvent)
		fmt.Printf("%s: deployment %s is %s\n", e.Action, e.Deployment.ID, e.Deployment.Status)
		if deployments.IsTerminal(e.Deployment.Status) {
			return
		}
	}
}
