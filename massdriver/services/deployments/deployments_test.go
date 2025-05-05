package deployments_test

import (
	"context"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/mockhttp"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/services/deployments"
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

func TestUpdateDeploymentStatus(t *testing.T) {
	tests := []struct {
		name            string
		status          int
		body            string
		expectErr       bool
		expectID        string
		expectErrType   error
		expectErrSubstr string
	}{
		{
			name:      "success",
			status:    200,
			body:      `{"id":"dep-123","status":"COMPLETED"}`,
			expectErr: false,
			expectID:  "dep-123",
		},
		{
			name:            "invalid transition",
			status:          422,
			body:            `{"errors":{"status":["cannot transition from COMPLETED to RUNNING"]}}`,
			expectErr:       true,
			expectErrType:   &deployments.InvalidTransitionError{},
			expectErrSubstr: "invalid deployment transition",
		},
		{
			name:            "unauthorized",
			status:          403,
			body:            `{"error":"forbidden"}`,
			expectErr:       true,
			expectErrSubstr: "403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.body})
			svc := deployments.NewService(client)

			dep, err := svc.UpdateDeploymentStatus(context.Background(), "dep-123", "COMPLETED")

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, dep)

				if tt.expectErrType != nil {
					require.ErrorAs(t, err, &tt.expectErrType)
				}

				if tt.expectErrSubstr != "" {
					require.Contains(t, err.Error(), tt.expectErrSubstr)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectID, dep.ID)
			}
		})
	}
}
