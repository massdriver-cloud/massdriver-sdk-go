package schemas_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/mockhttp"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/services/schemas"
	"github.com/stretchr/testify/require"
)

const testBaseURL = "https://api.massdriver.mock"

func setupTestService(t *testing.T) (*schemas.Service, *mockhttp.MockTransport) {
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

	service := schemas.NewService(testClient)
	return service, mockTransport
}

func TestNewService(t *testing.T) {
	testClient := &client.Client{}
	service := schemas.NewService(testClient)

	require.NotNil(t, service)
}

func TestService_GetArtifactDefinitionSchema(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		responseBody   string
		expectErr      bool
		expectedSchema map[string]interface{}
	}{
		{
			name:   "successful response",
			status: http.StatusOK,
			responseBody: `{
                "$schema": "http://json-schema.org/draft-07/schema#",
                "type": "object",
                "properties": {
                    "name": {"type": "string"}
                }
            }`,
			expectErr: false,
			expectedSchema: map[string]interface{}{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			name:         "server error",
			status:       http.StatusInternalServerError,
			responseBody: `{"error": "internal server error"}`,
			expectErr:    true,
		},
		{
			name:         "not found error",
			status:       http.StatusNotFound,
			responseBody: `{"error": "not found"}`,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTransport := setupTestService(t)

			mockTransport.RegisterResponse("GET", testBaseURL+"/json-schemas/artifact-definition.json", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			ctx := context.Background()
			schema, err := service.GetArtifactDefinitionSchema(ctx)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, schema)
			} else {
				require.NoError(t, err)
				require.NotNil(t, schema)
				require.Equal(t, tt.expectedSchema, map[string]interface{}(*schema))
			}

			require.Equal(t, 1, mockTransport.CallCount)
		})
	}
}

func TestService_GetBundleSchema(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		responseBody   string
		expectErr      bool
		expectedSchema map[string]interface{}
	}{
		{
			name:   "successful response",
			status: http.StatusOK,
			responseBody: `{
                "$schema": "http://json-schema.org/draft-07/schema#",
                "type": "object",
                "properties": {
                    "bundle": {"type": "object"}
                }
            }`,
			expectErr: false,
			expectedSchema: map[string]interface{}{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type":    "object",
				"properties": map[string]interface{}{
					"bundle": map[string]interface{}{"type": "object"},
				},
			},
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

			mockTransport.RegisterResponse("GET", testBaseURL+"/json-schemas/bundle.json", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			ctx := context.Background()
			schema, err := service.GetBundleSchema(ctx)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, schema)
			} else {
				require.NoError(t, err)
				require.NotNil(t, schema)
				require.Equal(t, tt.expectedSchema, map[string]interface{}(*schema))
			}

			require.Equal(t, 1, mockTransport.CallCount)
		})
	}
}

func TestService_GetMetaSchema(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		responseBody   string
		expectErr      bool
		expectedSchema map[string]interface{}
	}{
		{
			name:   "successful response",
			status: http.StatusOK,
			responseBody: `{
                "$schema": "http://json-schema.org/draft-07/schema#",
                "$id": "http://json-schema.org/draft-07/schema#",
                "type": "object"
            }`,
			expectErr: false,
			expectedSchema: map[string]interface{}{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"$id":     "http://json-schema.org/draft-07/schema#",
				"type":    "object",
			},
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

			mockTransport.RegisterResponse("GET", testBaseURL+"/json-schemas/draft-7.json", &mockhttp.MockHTTPResponse{
				StatusCode: tt.status,
				Body:       tt.responseBody,
			})

			ctx := context.Background()
			schema, err := service.GetMetaSchema(ctx)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, schema)
			} else {
				require.NoError(t, err)
				require.NotNil(t, schema)
				require.Equal(t, tt.expectedSchema, map[string]interface{}(*schema))
			}

			require.Equal(t, 1, mockTransport.CallCount)
		})
	}
}
