package schemas

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
)

type Schema map[string]interface{}

type Service struct {
	client *client.Client
}

// NewService creates a new Schemas service.
func NewService(c *client.Client) *Service {
	return &Service{client: c}
}

func (s *Service) GetArtifactDefinitionSchema(ctx context.Context) (*Schema, error) {
	var result Schema
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetResult(&result).
		Get("/json-schemas/artifact-definition.json")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("get artifact definition schema failed: %s", resp.Status())
	}
	return &result, nil
}

func (s *Service) GetBundleSchema(ctx context.Context) (*Schema, error) {
	var result Schema
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetResult(&result).
		Get("/json-schemas/bundle.json")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("get bundle schema failed: %s", resp.Status())
	}
	return &result, nil
}

func (s *Service) GetMetaSchema(ctx context.Context) (*Schema, error) {
	var result Schema
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetResult(&result).
		Get("/json-schemas/draft-7.json")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("get draft-7 schema failed: %s", resp.Status())
	}
	return &result, nil
}
