package deployments_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/mockhttp"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/services/deployments"
	"github.com/stretchr/testify/require"
)

const testBaseURL = "https://api.massdriver.mock"

func setupTestService(t *testing.T) (*deployments.Service, *mockhttp.MockTransport) {
	t.Helper()

	mockTransport := mockhttp.NewMockTransport()
	httpClient := resty.New().
		SetTransport(mockTransport).
		SetBaseURL(testBaseURL).
		SetHeader("Authorization", "Basic testtoken").
		SetHeader("Content-Type", "application/json")

	testClient := &client.Client{
		Config: config.Config{
			URL: testBaseURL,
		},
		HTTP: httpClient,
	}

	service := deployments.NewService(testClient)
	return service, mockTransport
}

func TestNewService(t *testing.T) {
	testClient := &client.Client{}
	service := deployments.NewService(testClient)

	require.NotNil(t, service)
}

func TestService_UpdateDeploymentStatus(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		expectErr    bool
		expectResult bool
	}{
		{
			name:         "successful status update",
			status:       http.StatusOK,
			responseBody: `{"id": "123", "status": "COMPLETED"}`,
			expectErr:    false,
			expectResult: true,
		},
		{
			name:         "invalid transition error",
			status:       http.StatusUnprocessableEntity,
			responseBody: `{"errors": {"status": ["Cannot transition from RUNNING to COMPLETED"]}}`,
			expectErr:    true,
			expectResult: false,
		},
		{
			name:         "deployment not found",
			status:       http.StatusNotFound,
			responseBody: `{"error": "deployment not found"}`,
			expectErr:    true,
			expectResult: false,
		},
		{
			name:         "invalid status",
			status:       http.StatusBadRequest,
			responseBody: `{"error": "invalid status"}`,
			expectErr:    true,
			expectResult: false,
		},
		{
			name:         "server error",
			status:       http.StatusInternalServerError,
			responseBody: `{"error": "internal server error"}`,
			expectErr:    true,
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTransport := setupTestService(t)

			mockTransport.RegisterResponse("PATCH", testBaseURL+"/v1/deployments/123", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			ctx := context.Background()
			result, err := service.UpdateDeploymentStatus(ctx, "123", deployments.StatusCompleted)

			if tt.expectErr {
				require.Error(t, err)
				if tt.status == http.StatusUnprocessableEntity {
					require.True(t, deployments.IsInvalidTransitionError(err))
				}
			} else {
				require.NoError(t, err)
			}

			if tt.expectResult {
				require.NotNil(t, result)
				require.Equal(t, "123", result.ID)
				require.Equal(t, "COMPLETED", result.Status)
			} else {
				require.Nil(t, result)
			}

			require.Equal(t, 1, mockTransport.CallCount)
		})
	}
}

func TestIsInvalidTransitionError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expectErr bool
	}{
		{
			name:      "is invalid transition error",
			err:       &deployments.InvalidTransitionError{Response: "test error"},
			expectErr: true,
		},
		{
			name:      "is not invalid transition error",
			err:       fmt.Errorf("regular error"),
			expectErr: false,
		},
		{
			name:      "nil error",
			err:       nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deployments.IsInvalidTransitionError(tt.err)
			require.Equal(t, tt.expectErr, result)
		})
	}
}
