package organizations_test

import (
	"context"
	"fmt"
	"log"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// StreamEvents subscribes to the organization's event feed: projects
// created, OCI repositories created in the bundle catalog, and bundle
// versions published to those repositories. The configured
// organization id is used implicitly.
//
// Cancel ctx to tear down the subscription and close the channel.
func ExampleService_StreamEvents() {
	c, err := massdriver.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := c.Organizations.StreamEvents(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for ev := range events {
		switch e := ev.(type) {
		case *types.ProjectEvent:
			fmt.Printf("%s: project %s\n", e.Action, e.Project.Name)
		case *types.OciRepoEvent:
			fmt.Printf("%s: repo %s\n", e.Action, e.OciRepo.Name)
		case *types.BundleEvent:
			fmt.Printf("%s: bundle %s@%s\n", e.Action, e.Bundle.Name, e.Bundle.Version)
		}
	}
}
