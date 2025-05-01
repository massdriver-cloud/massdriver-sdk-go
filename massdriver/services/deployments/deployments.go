package deployments

import (
	"context"
	"fmt"
	"net/http"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
)

type Status string

const (
	StatusPending   Status = "PENDING"
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
	StatusAborted   Status = "ABORTED"
)

type Deployment struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type Service struct {
	client *client.Client
}

func NewService(c *client.Client) *Service {
	return &Service{client: c}
}

// UpdateDeploymentStatus updates the deployment status (e.g., to "COMPLETED")
func (s *Service) UpdateDeploymentStatus(ctx context.Context, id string, status Status) (*Deployment, error) {
	var result Deployment

	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetBody(map[string]Status{"status": status}).
		SetResult(&result).
		Patch("/v1/deployments/" + id)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() == http.StatusUnprocessableEntity {
		return nil, fmt.Errorf("invalid deployment transition: %s", resp.String())
	}

	if resp.IsError() {
		return nil, fmt.Errorf("failed to update deployment: %s", resp.String())
	}

	return &result, nil
}
