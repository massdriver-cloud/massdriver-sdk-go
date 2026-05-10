package deployments_test

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/deployments"
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
