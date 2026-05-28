package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/rest"
)

// ErrNotFound is returned when the REST endpoint responds with HTTP 404.
// Match with [errors.Is].
var ErrNotFound = errors.New("not found")

// Resource is the create/update/show payload for /v1/resources.
//
// The server dispatches on presence of `field` in the body: bodies with
// `field` set go through the provisioned-artifact path (deployment token
// auth, type inferred from the deployment's package); bodies without it
// go through the imported-artifact path (requires `type`). `omitempty`
// on every optional field is what keeps these two flows distinct on the
// wire — do not remove it.
type Resource struct {
	ID      string                 `json:"id,omitempty"`
	Field   string                 `json:"field,omitempty"`
	Type    string                 `json:"type,omitempty"`
	Name    string                 `json:"name,omitempty"`
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
