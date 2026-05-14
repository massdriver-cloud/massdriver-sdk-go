package massdriver_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/auditlogs"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
)

// Showing the canonical "construct + first call" shape. Every other
// example assumes you've already done this.
func ExampleNewClient() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	v, err := c.Viewer.Get(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("authenticated as %s (%s)\n", v.Name, v.Kind)
}

// Distinguish "doesn't exist" from "denied" from "auth failed" without
// parsing error strings.
func ExampleClient_errorClassification() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	proj, err := c.Projects.Get(context.Background(), "ecommerce")
	switch {
	case errors.Is(err, gql.ErrNotFound):
		fmt.Println("project not found — has it been created?")
	case errors.Is(err, gql.ErrUnauthenticated):
		fmt.Println("credentials are invalid — re-authenticate")
	case errors.Is(err, gql.ErrForbidden):
		fmt.Println("authenticated, but the policy doesn't grant project:view")
	case err != nil:
		log.Fatal(err)
	default:
		fmt.Println(proj.Name)
	}
}

// Surface per-field validation errors a mutation rejected.
func ExampleClient_mutationValidation() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.Projects.Create(context.Background(), projects.CreateInput{
		// Intentionally invalid: missing required fields.
	})
	if mf, ok := gql.AsMutationFailedError(err); ok {
		for _, m := range mf.Messages {
			fmt.Printf("%s: %s (%s)\n", m.Field, m.Message, m.Code)
		}
		return
	}
	if err != nil {
		log.Fatal(err)
	}
}

// Iterate audit logs lazily — for queries where the result might span
// thousands of pages, [auditlogs.Service.Iter] avoids buffering the
// whole match in memory.
func ExampleClient_pagination() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	count := 0
	for ev, err := range c.AuditLogs.Iter(context.Background(), auditlogs.ListInput{
		Type: "deployment.completed",
	}) {
		if err != nil {
			log.Fatal(err)
		}
		count++
		if count >= 100 {
			break // stop after 100 — iterator stops fetching pages
		}
		_ = ev
	}
	fmt.Printf("processed %d events\n", count)
}

// Build a Client backed by a scripted GraphQL mock — for unit tests
// that exercise SDK methods without making real network calls.
func ExampleNewClient_test() {
	mock := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"project": map[string]any{
				"id":   "ecommerce",
				"name": "E-Commerce",
			},
		}),
	)

	c, err := massdriver.NewClient(
		massdriver.WithGQLClient(mock),
		massdriver.WithOrganizationID("test-org"),
	)
	if err != nil {
		log.Fatal(err)
	}

	proj, _ := c.Projects.Get(context.Background(), "ecommerce")
	fmt.Println(proj.Name)
	// Output: E-Commerce
}
