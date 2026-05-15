package policies_test

import (
	"context"
	"fmt"
	"log"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/policies"
)

// Attach an ABAC policy that restricts a group's project access to
// the production environment.
func ExampleService_Create() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	p, err := c.Policies.Create(context.Background(), "platform-engineers", policies.CreatePolicyInput{
		Effect:  policies.EffectAllow,
		Actions: []string{"project:view", "deployment:create"},
		Conditions: policies.PolicyConditions{
			"md-environment": {"production"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(p.ID)
}

// A wildcard policy — nil Conditions matches every entity.
func ExampleService_Create_wildcard() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.Policies.Create(context.Background(), "admins", policies.CreatePolicyInput{
		Effect:     policies.EffectAllow,
		Actions:    []string{"project:view", "deployment:view"},
		Conditions: nil, // wildcard — every project, every environment
	})
	if err != nil {
		log.Fatal(err)
	}
}

// Service.ListActions returns the runtime catalog. Useful when
// rendering a UI for policy authors.
func ExampleService_ListActions() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	actions, err := c.Policies.ListActions(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range actions {
		fmt.Printf("%s — %s\n", a.ID, a.Description)
	}
}
