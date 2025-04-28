package artifacts

import (
	"context"
	"fmt"
	"net/http"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
)

type Artifact struct {
	ID       string                 `json:"id,omitempty"`
	Metadata *Metadata              `json:"metadata"`
	Data     map[string]interface{} `json:"data"`
	Specs    map[string]interface{} `json:"specs"`
}

type Metadata struct {
	Field              string `json:"field"`
	ProviderResourceID string `json:"provider_resource_id"`
	Type               string `json:"type"`
	Name               string `json:"name"`
}

type Service struct {
	client *client.Client
}

// NewService creates a new Artifact service.
func NewService(c *client.Client) *Service {
	return &Service{client: c}
}

// CreateArtifact sends a POST /artifacts request
func (s *Service) CreateArtifact(ctx context.Context, a Artifact) (*Artifact, error) {
	var result Artifact
	resp, err := s.client.HTTP.R().
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
func (s *Service) GetArtifact(ctx context.Context, id string) (*Artifact, error) {
	var result Artifact
	resp, err := s.client.HTTP.R().
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
func (s *Service) UpdateArtifact(ctx context.Context, id string, a Artifact) (*Artifact, error) {
	var result Artifact
	resp, err := s.client.HTTP.R().
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
func (s *Service) DeleteArtifact(ctx context.Context, id, field string) error {
	payload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"field": field,
		},
	}
	resp, err := s.client.HTTP.R().
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
