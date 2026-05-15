package projects_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
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
