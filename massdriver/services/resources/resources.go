package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
)

type Resource struct {
	ID      string                 `json:"id"`
	Field   string                 `json:"field"`
	Type    string                 `json:"type"`
	Name    string                 `json:"name"`
	Payload map[string]interface{} `json:"payload"`
}

type Service struct {
	client *client.Client
}

// NewService creates a new Resource service.
func NewService(c *client.Client) *Service {
	return &Service{client: c}
}

// CreateResource sends a POST /v1/resources request
func (s *Service) CreateResource(ctx context.Context, a *Resource) (*Resource, error) {
	var result Resource
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetBody(a).
		SetResult(&result).
		Post("/v1/resources")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("create resource failed: %s", resp.Status())
	}
	return &result, nil
}

// GetResource sends a GET /v1/resources/:id request
func (s *Service) GetResource(ctx context.Context, id string) (*Resource, error) {
	var result Resource
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetResult(&result).
		Get("/v1/resources/" + id)

	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, errors.New("not found")
	}
	if resp.IsError() {
		return nil, fmt.Errorf("get resource failed: %s", resp.Status())
	}
	return &result, nil
}

// UpdateResource sends a PUT /v1/resources/:id request
func (s *Service) UpdateResource(ctx context.Context, id string, a *Resource) (*Resource, error) {
	var result Resource
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetBody(a).
		SetResult(&result).
		Put("/v1/resources/" + id)

	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, errors.New("not found")
	}
	if resp.IsError() {
		return nil, fmt.Errorf("update resource failed: %s", resp.Status())
	}
	return &result, nil
}

// DeleteResource sends a DELETE /v1/resources/:id with metadata.field in the body
func (s *Service) DeleteResource(ctx context.Context, id, field string) error {
	payload := map[string]interface{}{
		"field": field,
	}
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetBody(payload).
		Delete("/v1/resources/" + id)

	if err != nil {
		return err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return errors.New("not found")
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("delete resource failed: %s", resp.Status())
	}
	return nil
}
