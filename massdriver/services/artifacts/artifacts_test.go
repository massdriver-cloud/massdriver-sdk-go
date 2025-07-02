package artifacts_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/mockhttp"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/services/artifacts"
	"github.com/stretchr/testify/require"
)

const testBaseURL = "https://api.massdriver.mock"

func setupTestService(t *testing.T) (*artifacts.Service, *mockhttp.MockTransport) {
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

	service := artifacts.NewService(testClient)
	return service, mockTransport
}

func TestNewService(t *testing.T) {
	testClient := &client.Client{}
	service := artifacts.NewService(testClient)

	require.NotNil(t, service)
}

func TestService_CreateArtifact(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		expectErr    bool
	}{
		{
			name:         "successful creation",
			status:       http.StatusCreated,
			responseBody: `{"id": "123", "name": "test-artifact", "type": "kubernetes-cluster"}`,
			expectErr:    false,
		},
		{
			name:         "validation error",
			status:       http.StatusBadRequest,
			responseBody: `{"error": "invalid artifact data"}`,
			expectErr:    true,
		},
		{
			name:         "server error",
			status:       http.StatusInternalServerError,
			responseBody: `{"error": "internal server error"}`,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTransport := setupTestService(t)

			mockTransport.RegisterResponse("POST", testBaseURL+"/v1/artifacts", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			artifact := &artifacts.Artifact{
				Name: "test-artifact",
				Type: "kubernetes-cluster",
				Data: map[string]interface{}{"test": "data"},
			}

			ctx := context.Background()
			result, err := service.CreateArtifact(ctx, artifact)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}

			require.Equal(t, 1, mockTransport.CallCount)
		})
	}
}

func TestService_GetArtifact(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		expectErr    bool
	}{
		{
			name:         "successful get",
			status:       http.StatusOK,
			responseBody: `{"id": "123", "name": "test-artifact", "type": "kubernetes-cluster"}`,
			expectErr:    false,
		},
		{
			name:         "not found",
			status:       http.StatusNotFound,
			responseBody: `{"error": "artifact not found"}`,
			expectErr:    true,
		},
		{
			name:         "server error",
			status:       http.StatusInternalServerError,
			responseBody: `{"error": "internal server error"}`,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTransport := setupTestService(t)

			mockTransport.RegisterResponse("GET", testBaseURL+"/v1/artifacts/123", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			ctx := context.Background()
			result, err := service.GetArtifact(ctx, "123")

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}

			require.Equal(t, 1, mockTransport.CallCount)
		})
	}
}

func TestService_UpdateArtifact(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		expectErr    bool
	}{
		{
			name:         "successful update",
			status:       http.StatusOK,
			responseBody: `{"id": "123", "name": "updated-artifact", "type": "kubernetes-cluster"}`,
			expectErr:    false,
		},
		{
			name:         "not found",
			status:       http.StatusNotFound,
			responseBody: `{"error": "artifact not found"}`,
			expectErr:    true,
		},
		{
			name:         "validation error",
			status:       http.StatusBadRequest,
			responseBody: `{"error": "invalid artifact data"}`,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTransport := setupTestService(t)

			mockTransport.RegisterResponse("PUT", testBaseURL+"/v1/artifacts/123", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			artifact := &artifacts.Artifact{
				ID:   "123",
				Name: "updated-artifact",
				Type: "kubernetes-cluster",
				Data: map[string]interface{}{"test": "updated"},
			}

			ctx := context.Background()
			result, err := service.UpdateArtifact(ctx, "123", artifact)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}

			require.Equal(t, 1, mockTransport.CallCount)
		})
	}
}

func TestService_DeleteArtifact(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		expectErr    bool
	}{
		{
			name:         "successful deletion",
			status:       http.StatusOK,
			responseBody: `{}`,
			expectErr:    false,
		},
		{
			name:         "not found",
			status:       http.StatusNotFound,
			responseBody: `{"error": "artifact not found"}`,
			expectErr:    true,
		},
		{
			name:         "server error",
			status:       http.StatusInternalServerError,
			responseBody: `{"error": "internal server error"}`,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTransport := setupTestService(t)

			mockTransport.RegisterResponse("DELETE", testBaseURL+"/v1/artifacts/123", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			ctx := context.Background()
			err := service.DeleteArtifact(ctx, "123", "test-field")

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, 1, mockTransport.CallCount)
		})
	}
}
