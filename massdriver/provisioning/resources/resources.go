package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/rest"
)

// ErrNotFound is returned when the REST endpoint responds with HTTP 404.
// Match with [errors.Is].
var ErrNotFound = errors.New("not found")

// Resource is a resource as returned by /v1/resources.
type Resource struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// Type is the legacy `<org>/<identifier>` name and carries no version;
	// ResourceType is `identifier@version` with the resolved version.
	Type         string `json:"type"`
	ResourceType string `json:"resource_type"`

	// VersionConstraint is the range the producing bundle declared for this
	// field ("~1", "1.2.3", "latest"), empty if it declared none.
	VersionConstraint string `json:"version_constraint"`

	Field     string         `json:"field"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// ResourceInput is the body of a create or update request to /v1/resources.
// All three fields are required; the resource type is resolved server-side
// from the deployment's release pin and Field.
type ResourceInput struct {
	Field   string         `json:"field"`
	Name    string         `json:"name"`
	Payload map[string]any `json:"payload"`
}

type Service struct {
	client *client.Client
}

// NewService creates a new Resource service.
func NewService(c *client.Client) *Service {
	return &Service{client: c}
}

// CreateResource sends a POST /v1/resources request
func (s *Service) CreateResource(ctx context.Context, input *ResourceInput) (*Resource, error) {
	var result Resource
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetBody(input).
		SetResult(&result).
		Post("/v1/resources")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, formatAPIError("create resource", resp)
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
		return nil, fmt.Errorf("get resource %s: %w", id, ErrNotFound)
	}
	if resp.IsError() {
		return nil, formatAPIError("get resource "+id, resp)
	}
	return &result, nil
}

// UpdateResource sends a PUT /v1/resources/:id request
func (s *Service) UpdateResource(ctx context.Context, id string, input *ResourceInput) (*Resource, error) {
	var result Resource
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetBody(input).
		SetResult(&result).
		Put("/v1/resources/" + id)

	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("update resource %s: %w", id, ErrNotFound)
	}
	if resp.IsError() {
		return nil, formatAPIError("update resource "+id, resp)
	}
	return &result, nil
}

// DeleteResource sends a DELETE /v1/resources/:id request.
func (s *Service) DeleteResource(ctx context.Context, id string) error {
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		Delete("/v1/resources/" + id)

	if err != nil {
		return err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return fmt.Errorf("delete resource %s: %w", id, ErrNotFound)
	}
	if resp.IsError() {
		return formatAPIError("delete resource "+id, resp)
	}
	return nil
}

// formatAPIError builds an error that includes the server's JSON error
// body when one is present, so callers see e.g. "payload: can't be blank"
// instead of a bare "422 Unprocessable Entity".
func formatAPIError(op string, resp *resty.Response) error {
	errResp, parseErr := rest.ParseJSONErrorResponse(resp)
	if parseErr != nil {
		return fmt.Errorf("%s: %d: %s", op, resp.StatusCode(), strings.TrimSpace(string(resp.Body())))
	}
	if errResp.IsValidationError() {
		fields := make([]string, 0, len(errResp.Errors))
		for k := range errResp.Errors {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		parts := make([]string, 0, len(fields))
		for _, k := range fields {
			parts = append(parts, fmt.Sprintf("%s: %s", k, strings.Join(errResp.Errors[k], ", ")))
		}
		return fmt.Errorf("%s: %d: %s", op, resp.StatusCode(), strings.Join(parts, "; "))
	}
	if errResp.IsFrameworkError() {
		return fmt.Errorf("%s: %d: %s", op, resp.StatusCode(), errResp.Error)
	}
	return fmt.Errorf("%s: %d", op, resp.StatusCode())
}
