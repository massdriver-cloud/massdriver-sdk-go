package resources_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/resources"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func ExampleService_Get() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	r, err := c.Resources.Get(context.Background(), "res-auth-prod")
	if errors.Is(err, gql.ErrNotFound) {
		fmt.Println("resource not found")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	// Sensitive fields are masked as "[SENSITIVE]" in this payload —
	// use Service.Export to retrieve the unmasked values.
	fmt.Printf("%s (%s)\n", r.Name, r.Origin)
}

func ExampleService_Export() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	// Export is recorded in the audit log so access to credentials is
	// attributable. Pass FormatJSON for the raw payload, or any
	// resource-type-declared format (e.g., "yaml", "env").
	exp, err := c.Resources.Export(context.Background(), "res-auth-prod", resources.FormatJSON)
	if err != nil {
		log.Fatal(err)
	}
	// Embedded Resource has the unmasked Payload; Rendered is the
	// server-rendered string in the requested format.
	fmt.Printf("%s payload keys: %d\n", exp.Name, len(exp.Payload))
	fmt.Println(exp.Rendered)
}

func ExampleService_CreateGrant() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	// Share the resource with environments tagged team=platform.
	grant, err := c.Resources.CreateGrant(context.Background(), "res-shared-secret", resources.CreateGrantInput{
		Action: "resource:export",
		RecipientConditions: types.PolicyConditions{
			"team": {"platform"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(grant.ID)
}
