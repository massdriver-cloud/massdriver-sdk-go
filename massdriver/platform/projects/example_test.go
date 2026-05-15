package projects_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func ExampleService_Get() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	proj, err := c.Projects.Get(context.Background(), "ecommerce")
	if errors.Is(err, gql.ErrNotFound) {
		fmt.Println("project not found")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s — %d environments\n", proj.Name, len(proj.Environments))
}

// StreamEvents subscribes to a project's event feed: changes to the
// project itself, lifecycle events for its environments, and edits to
// its blueprint (components added/removed, links wired/unwired).
//
// Cancel ctx to tear down the subscription and close the channel.
func ExampleService_StreamEvents() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := c.Projects.StreamEvents(ctx, "ecommerce")
	if err != nil {
		log.Fatal(err)
	}
	for ev := range events {
		switch e := ev.(type) {
		case *types.ProjectEvent:
			fmt.Printf("%s: project %s\n", e.Action, e.Project.Name)
		case *types.EnvironmentEvent:
			fmt.Printf("%s: environment %s\n", e.Action, e.Environment.Name)
		case *types.ComponentEvent:
			fmt.Printf("%s: component %s\n", e.Action, e.Component.Name)
		case *types.LinkEvent:
			fmt.Printf("%s: link %s → %s\n", e.Action, e.Link.FromField, e.Link.ToField)
		}
	}
}

func ExampleService_Create() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	proj, err := c.Projects.Create(context.Background(), projects.CreateInput{
		ID:          "ecommerce",
		Name:        "E-Commerce",
		Description: "Customer-facing storefront and APIs.",
		Attributes: map[string]any{
			"team": "platform",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(proj.ID)
}
