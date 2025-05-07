package packagealarms

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
)

type Dimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Metric struct {
	Name       string      `json:"name"`
	Namespace  string      `json:"namespace"`
	Statistic  string      `json:"statistic"`
	Dimensions []Dimension `json:"dimensions,omitempty"`
}

type Alarm struct {
	ID                 string  `json:"id,omitempty"`
	DisplayName        string  `json:"display_name"`
	CloudResourceID    string  `json:"cloud_resource_id"`
	Metric             *Metric `json:"metric,omitempty"`
	Threshold          float64 `json:"threshold,omitempty"`
	PeriodMinutes      int     `json:"period_minutes,omitempty"`
	ComparisonOperator string  `json:"comparsion_operator,omitempty"`
}

type Service struct {
	client *client.Client
}

func NewService(c *client.Client) *Service {
	return &Service{client: c}
}

// Create creates a new package alarm for a specific package.
func (s *Service) CreatePackageAlarm(ctx context.Context, packageID string, alarm *Alarm) (*Alarm, error) {
	var result Alarm
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetBody(alarm).
		SetResult(&result).
		Post(fmt.Sprintf("/v1/packages/%s/alarms", packageID))

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("create alarm failed: %s", resp.Status())
	}
	return &result, nil
}

// Get retrieves a package alarm by package ID and alarm ID.
func (s *Service) GetPackageAlarm(ctx context.Context, packageID, alarmID string) (*Alarm, error) {
	var result Alarm
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetResult(&result).
		Get(fmt.Sprintf("/v1/packages/%s/alarms/%s", packageID, alarmID))

	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, errors.New("not found")
	}
	if resp.IsError() {
		return nil, fmt.Errorf("get alarm failed: %s", resp.Status())
	}
	return &result, nil
}

// Update modifies an existing package alarm by ID.
func (s *Service) UpdatePackageAlarm(ctx context.Context, packageID, alarmID string, update *Alarm) (*Alarm, error) {
	var result Alarm
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		SetBody(update).
		SetResult(&result).
		Put(fmt.Sprintf("/v1/packages/%s/alarms/%s", packageID, alarmID))

	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, errors.New("not found")
	}
	if resp.IsError() {
		return nil, fmt.Errorf("update alarm failed: %s", resp.Status())
	}
	return &result, nil
}

// Delete removes a package alarm by ID.
func (s *Service) DeletePackageAlarm(ctx context.Context, packageID, alarmID string) error {
	resp, err := s.client.HTTP.R().
		SetContext(ctx).
		Delete(fmt.Sprintf("/v1/packages/%s/alarms/%s", packageID, alarmID))

	if err != nil {
		return err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return errors.New("not found")
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("delete alarm failed: %s", resp.Status())
	}
	return nil
}
