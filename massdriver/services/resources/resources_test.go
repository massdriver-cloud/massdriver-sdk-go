package resources_test

import (
	"context"
	"io"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/mockhttp"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/services/resources"
	"github.com/stretchr/testify/require"
)

func newTestClient(r *mockhttp.MockHTTPResponse) (*client.Client, *mockhttp.MutableRoundTripper) {
	roundtripper := mockhttp.MutableRoundTripper{Response: r}
	httpClient := resty.New().
		SetTransport(&roundtripper).
		SetBaseURL("https://api.massdriver.mock").
		SetHeader("Authorization", "Basic testtoken").
		SetHeader("Content-Type", "application/json")

	return &client.Client{
		HTTP: httpClient,
	}, &roundtripper
}

func TestCreateResource(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		sentBody     string
		responseBody string
		expectErr    bool
		expectID     string
	}{
		{
			name:         "success",
			status:       201,
			sentBody:     `{"id":"abc-123","name":"Created","field":"database","type":"db","payload":{"foo":"bar","key":"value"}}`,
			responseBody: `{"id":"abc-123","name":"Created"}`,
			expectID:     "abc-123",
			expectErr:    false,
		},
		{
			name:         "failure",
			status:       500,
			responseBody: `{"error":"something went wrong"}`,
			expectErr:    true,
		},
	}

	input := resources.Resource{
		ID:      "abc-123",
		Name:    "Created",
		Type:    "db",
		Field:   "database",
		Payload: map[string]interface{}{"foo": "bar", "key": "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, roundTripper := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.responseBody})
			service := resources.NewService(client)

			result, err := service.CreateResource(context.Background(), &input)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tt.expectID, result.ID)

				gotBody, err := io.ReadAll(roundTripper.ReceivedRequest.Body)
				require.NoError(t, err)
				require.JSONEq(t, tt.sentBody, string(gotBody))
			}
		})
	}
}

func TestGetResource(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		expectErr    bool
		expectNil    bool
		expectID     string
	}{
		{
			name:         "success",
			status:       200,
			responseBody: `{"id":"abc-123","name":"Fetched"}`,
			expectID:     "abc-123",
			expectErr:    false,
			expectNil:    false,
		},
		{
			name:         "not found",
			status:       404,
			responseBody: `{"error":"not found"}`,
			expectErr:    true,
			expectNil:    true,
		},
		{
			name:         "server error",
			status:       500,
			responseBody: `{"error":"fail"}`,
			expectErr:    true,
			expectNil:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.responseBody})
			service := resources.NewService(client)

			result, err := service.GetResource(context.Background(), "any-id")

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.expectNil {
					require.Nil(t, result)
				} else {
					require.Equal(t, tt.expectID, result.ID)
				}
			}
		})
	}
}

func TestUpdateResource(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		sentBody     string
		responseBody string
		expectErr    bool
		expectID     string
	}{
		{
			name:         "success",
			status:       200,
			sentBody:     `{"id":"xyz-789","name":"Updated","field":"database","type":"db","payload":{"bar":"baz","x":"y"}}`,
			responseBody: `{"id":"xyz-789","name":"Updated"}`,
			expectErr:    false,
			expectID:     "xyz-789",
		},
		{
			name:         "failure",
			status:       422,
			responseBody: `{"error":"invalid input"}`,
			expectErr:    true,
		},
	}

	input := resources.Resource{
		ID:      "xyz-789",
		Name:    "Updated",
		Field:   "database",
		Type:    "db",
		Payload: map[string]interface{}{"bar": "baz", "x": "y"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, roundTripper := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.responseBody})
			service := resources.NewService(client)

			result, err := service.UpdateResource(context.Background(), "xyz-789", &input)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectID, result.ID)

				gotBody, err := io.ReadAll(roundTripper.ReceivedRequest.Body)
				require.NoError(t, err)
				require.JSONEq(t, tt.sentBody, string(gotBody))
			}
		})
	}
}

func TestDeleteResource(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		sentBody     string
		responseBody string
		expectErr    bool
	}{
		{
			name:      "success",
			status:    200,
			sentBody:  `{"field":"db"}`,
			expectErr: false,
		},
		{
			name:         "failure",
			status:       400,
			responseBody: `{"error":"bad input"}`,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, roundTripper := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.responseBody})
			service := resources.NewService(client)

			err := service.DeleteResource(context.Background(), "abc-123", "db")

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				gotBody, err := io.ReadAll(roundTripper.ReceivedRequest.Body)
				require.NoError(t, err)
				require.JSONEq(t, tt.sentBody, string(gotBody))
			}
		})
	}
}
