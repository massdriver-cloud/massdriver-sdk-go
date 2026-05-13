package instances_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/instances"
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
