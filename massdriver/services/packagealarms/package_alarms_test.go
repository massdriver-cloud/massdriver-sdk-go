package packagealarms_test

import (
	"context"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/mockhttp"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/services/packagealarms"
	"github.com/stretchr/testify/require"
)

func newTestClient(r *mockhttp.MockHTTPResponse) *client.Client {
	httpClient := resty.New().
		SetTransport(&mockhttp.MutableRoundTripper{Response: r}).
		SetBaseURL("https://api.massdriver.mock").
		SetHeader("Authorization", "Basic testtoken").
		SetHeader("Content-Type", "application/json")

	return &client.Client{
		HTTP: httpClient,
	}
}

func TestCreateAlarm(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		expectErr bool
		expectID  string
	}{
		{
			name:      "success",
			status:    201,
			body:      `{"id": "alarm-123", "display_name": "CPU High", "cloud_resource_id": "arn"}`,
			expectErr: false,
			expectID:  "alarm-123",
		},
		{
			name:      "failure",
			status:    500,
			body:      `{"error":"something broke"}`,
			expectErr: true,
		},
	}

	input := packagealarms.Alarm{
		DisplayName:     "CPU High",
		CloudResourceID: "arn",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.body})
			svc := packagealarms.NewService(client)

			result, err := svc.CreatePackageAlarm(context.Background(), "pkg-abc", &input)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectID, result.ID)
			}
		})
	}
}

func TestGetAlarm(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		expectErr bool
		expectNil bool
		expectID  string
	}{
		{
			name:      "found",
			status:    200,
			body:      `{"id": "alarm-xyz", "display_name": "CPU High"}`,
			expectID:  "alarm-xyz",
			expectErr: false,
			expectNil: false,
		},
		{
			name:      "not found",
			status:    404,
			body:      `{}`,
			expectErr: false,
			expectNil: true,
		},
		{
			name:      "server error",
			status:    500,
			body:      `{"error":"fail"}`,
			expectErr: true,
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.body})
			svc := packagealarms.NewService(client)

			result, err := svc.GetPackageAlarm(context.Background(), "pkg-abc", "alarm-xyz")

			if tt.expectErr {
				require.Error(t, err)
			} else if tt.expectNil {
				require.NoError(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectID, result.ID)
			}
		})
	}
}

func TestUpdateAlarm(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		expectErr bool
		expectID  string
	}{
		{
			name:      "success",
			status:    200,
			body:      `{"id": "alarm-xyz", "display_name": "Updated"}`,
			expectErr: false,
			expectID:  "alarm-xyz",
		},
		{
			name:      "failure",
			status:    422,
			body:      `{"error":"invalid input"}`,
			expectErr: true,
		},
	}

	input := packagealarms.Alarm{
		DisplayName:     "Updated",
		CloudResourceID: "arn",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.body})
			svc := packagealarms.NewService(client)

			result, err := svc.UpdatePackageAlarm(context.Background(), "pkg-abc", "alarm-xyz", &input)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectID, result.ID)
			}
		})
	}
}

func TestDeleteAlarm(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		expectErr bool
	}{
		{
			name:      "success",
			status:    200,
			expectErr: false,
		},
		{
			name:      "failure",
			status:    400,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: `{}`})
			svc := packagealarms.NewService(client)

			err := svc.DeletePackageAlarm(context.Background(), "pkg-abc", "alarm-xyz")
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
