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
	Field string `json:"field"`
	Type  string `json:"type"`
	Name  string `json:"name"`
}

type Service struct {
	client *client.Client
}

// NewService creates a new Artifact service.
func NewService(c *client.Client) *Service {
	return &Service{client: c}
}

// CreateArtifact sends a POST /v1/artifacts request
func (s *Service) CreateArtifact(ctx context.Context, a *Artifact) (*Artifact, error) {
	var result Artifact
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetBody(a).
		SetResult(&result).
		Post("/v1/artifacts")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("create artifact failed: %s", resp.Status())
	}
	return &result, nil
}

// GetArtifact sends a GET /v1/artifacts/:id request
func (s *Service) GetArtifact(ctx context.Context, id string) (*Artifact, error) {
	var result Artifact
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetResult(&result).
		Get("/v1/artifacts/" + id)

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

// UpdateArtifact sends a PUT /v1/artifacts/:id request
func (s *Service) UpdateArtifact(ctx context.Context, id string, a *Artifact) (*Artifact, error) {
	var result Artifact
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetBody(a).
		SetResult(&result).
		Put("/v1/artifacts/" + id)

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("update artifact failed: %s", resp.Status())
	}
	return &result, nil
}

// DeleteArtifact sends a DELETE /v1/artifacts/:id with metadata.field in the body
func (s *Service) DeleteArtifact(ctx context.Context, id string) error {
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		Delete("/v1/artifacts/" + id)

	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("delete artifact failed: %s", resp.Status())
	}
	return nil
}
