package packagealarms_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/mockhttp"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/services/packagealarms"
	"github.com/stretchr/testify/require"
)

const testBaseURL = "https://api.massdriver.mock"

func setupTestService(t *testing.T) (*packagealarms.Service, *mockhttp.MockTransport) {
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

	service := packagealarms.NewService(testClient)
	return service, mockTransport
}

func TestNewService(t *testing.T) {
	testClient := &client.Client{}
	service := packagealarms.NewService(testClient)

	require.NotNil(t, service)
}

func TestService_CreatePackageAlarm(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		expectErr    bool
	}{
		{
			name:         "successful creation",
			status:       http.StatusCreated,
			responseBody: `{"id": "123", "display_name": "test-alarm", "cloud_resource_id": "resource-123"}`,
			expectErr:    false,
		},
		{
			name:         "validation error",
			status:       http.StatusBadRequest,
			responseBody: `{"error": "invalid alarm data"}`,
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

			mockTransport.RegisterResponse("POST", testBaseURL+"/v1/packages/pkg-123/alarms", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			alarm := &packagealarms.Alarm{
				DisplayName:     "test-alarm",
				CloudResourceID: "resource-123",
			}

			ctx := context.Background()
			result, err := service.CreatePackageAlarm(ctx, "pkg-123", alarm)

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

func TestService_GetPackageAlarm(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		expectErr    bool
	}{
		{
			name:         "successful get",
			status:       http.StatusOK,
			responseBody: `{"id": "123", "display_name": "test-alarm", "cloud_resource_id": "resource-123"}`,
			expectErr:    false,
		},
		{
			name:         "not found",
			status:       http.StatusNotFound,
			responseBody: `{"error": "alarm not found"}`,
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

			mockTransport.RegisterResponse("GET", testBaseURL+"/v1/packages/pkg-123/alarms/123", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			ctx := context.Background()
			result, err := service.GetPackageAlarm(ctx, "pkg-123", "123")

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

func TestService_UpdatePackageAlarm(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		expectErr    bool
	}{
		{
			name:         "successful update",
			status:       http.StatusOK,
			responseBody: `{"id": "123", "display_name": "updated-alarm", "cloud_resource_id": "resource-123"}`,
			expectErr:    false,
		},
		{
			name:         "not found",
			status:       http.StatusNotFound,
			responseBody: `{"error": "alarm not found"}`,
			expectErr:    true,
		},
		{
			name:         "validation error",
			status:       http.StatusBadRequest,
			responseBody: `{"error": "invalid alarm data"}`,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTransport := setupTestService(t)

			mockTransport.RegisterResponse("PUT", testBaseURL+"/v1/packages/pkg-123/alarms/123", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			alarm := &packagealarms.Alarm{
				ID:              "123",
				DisplayName:     "updated-alarm",
				CloudResourceID: "resource-123",
			}

			ctx := context.Background()
			result, err := service.UpdatePackageAlarm(ctx, "pkg-123", "123", alarm)

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

func TestService_DeletePackageAlarm(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		expectErr    bool
	}{
		{
			name:         "successful deletion",
			status:       http.StatusOK,
			responseBody: "",
			expectErr:    false,
		},
		{
			name:         "not found",
			status:       http.StatusNotFound,
			responseBody: `{"error": "alarm not found"}`,
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

			mockTransport.RegisterResponse("DELETE", testBaseURL+"/v1/packages/pkg-123/alarms/123", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			ctx := context.Background()
			err := service.DeletePackageAlarm(ctx, "pkg-123", "123")

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, 1, mockTransport.CallCount)
		})
	}
}
