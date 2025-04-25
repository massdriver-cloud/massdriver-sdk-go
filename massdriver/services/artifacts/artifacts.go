package artifacts

import (
	"context"
	"fmt"
	"net/http"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
)

type Artifact struct {
	ID       string                 `json:"id,omitempty"`
	Metadata map[string]interface{} `json:"metadata"`
	Data     map[string]interface{} `json:"data"`
	Specs    map[string]interface{} `json:"specs"`
}

// CreateArtifact sends a POST /artifacts request
func CreateArtifact(ctx context.Context, c *client.Client, a Artifact) (*Artifact, error) {
	var result Artifact
	resp, err := c.HTTP.R().
		SetContext(ctx).
		SetBody(a).
		SetResult(&result).
		Post("/artifacts")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("create artifact failed: %s", resp.Status())
	}
	return &result, nil
}

// GetArtifact sends a GET /artifacts/:id request
func GetArtifact(ctx context.Context, c *client.Client, id string) (*Artifact, error) {
	var result Artifact
	resp, err := c.HTTP.R().
		SetContext(ctx).
		SetResult(&result).
		Get("/artifacts/" + id)

	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, nil
	}
	if resp.IsError() {
		return nil, fmt.Errorf("get artifact failed: %s", resp.Status())
	}
	return &result, nil
}

// UpdateArtifact sends a PUT /artifacts/:id request
func UpdateArtifact(ctx context.Context, c *client.Client, id string, a Artifact) (*Artifact, error) {
	var result Artifact
	resp, err := c.HTTP.R().
		SetContext(ctx).
		SetBody(a).
		SetResult(&result).
		Put("/artifacts/" + id)

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("update artifact failed: %s", resp.Status())
	}
	return &result, nil
}

// DeleteArtifact sends a DELETE /artifacts/:id with metadata.field in the body
func DeleteArtifact(ctx context.Context, c *client.Client, id, field string) error {
	payload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"field": field,
		},
	}
	resp, err := c.HTTP.R().
		SetContext(ctx).
		SetBody(payload).
		Delete("/artifacts/" + id)

	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusNoContent {
		return fmt.Errorf("delete artifact failed: %s", resp.Status())
	}
	return nil
}
