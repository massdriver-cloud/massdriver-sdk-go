package instances_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/instances"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func ExampleService_Get() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	inst, err := c.Instances.Get(context.Background(), "ecomm-prod-database")
	if errors.Is(err, gql.ErrNotFound) {
		fmt.Println("instance not found")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s — version %s, status %s\n", inst.Name, inst.ResolvedVersion, inst.Status)
}

func ExampleService_Update() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	// Pin the instance to a new release channel. Takes effect on the
	// next deployment — ResolvedVersion updates immediately;
	// DeployedVersion only changes once a deploy runs.
	inst, err := c.Instances.Update(context.Background(), "ecomm-prod-database", instances.UpdateInput{
		Version: "~2.0",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(inst.ResolvedVersion)
}

func ExampleService_Orphan() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	// Break-glass: an instance is permanently stuck and cannot be
	// recovered through deployments. Orphan clears the state locks and
	// bulk-aborts any RUNNING/PENDING/APPROVED/FAILED deployments so the
	// instance settles back to INITIALIZED.
	//
	// Leave DeleteState false unless the remote Terraform/OpenTofu state
	// is known to be unrecoverable — DeleteState: true is irreversible
	// and the next deployment may duplicate previously-tracked resources.
	inst, err := c.Instances.Orphan(context.Background(), "ecomm-prod-database", instances.OrphanInput{
		DeleteState: false,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(inst.Status)
}

// StreamEvents subscribes to an instance's event feed: config changes,
// new deployments, incoming connection wires, and alarm-state
// transitions all arrive on a single channel. Each frame is one of a
// handful of concrete event types — type-switch to read the payload.
//
// Cancel ctx to tear down the subscription and close the channel.
func ExampleService_StreamEvents() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := c.Instances.StreamEvents(ctx, "ecomm-prod-database")
	if err != nil {
		log.Fatal(err)
	}
	for ev := range events {
		switch e := ev.(type) {
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

func ExampleService_SetSecret() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	// Secret values are encrypted at rest and never returned by the API
	// — the call returns metadata only (name, fingerprint, timestamps).
	if _, err := c.Instances.SetSecret(context.Background(), "ecomm-prod-database", "DATABASE_PASSWORD", "s3cret"); err != nil {
		log.Fatal(err)
	}
}
