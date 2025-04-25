package artifact_test

import (
	"context"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/mockhttp"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/services/artifact"
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

func TestCreateArtifact(t *testing.T) {
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
			body:      `{"id":"abc-123","metadata":{"name":"Created"}}`,
			expectID:  "abc-123",
			expectErr: false,
		},
		{
			name:      "failure",
			status:    500,
			body:      `{"error":"something went wrong"}`,
			expectErr: true,
		},
	}

	input := artifact.Artifact{
		Metadata: &artifact.Metadata{Name: "Created"},
		Data:     map[string]interface{}{"foo": "bar"},
		Specs:    map[string]interface{}{"key": "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.body})
			result, err := artifact.CreateArtifact(context.Background(), client, input)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tt.expectID, result.ID)
			}
		})
	}
}

func TestGetArtifact(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		expectErr bool
		expectNil bool
		expectID  string
	}{
		{
			name:      "success",
			status:    200,
			body:      `{"id":"abc-123","metadata":{"name":"Fetched"}}`,
			expectID:  "abc-123",
			expectErr: false,
			expectNil: false,
		},
		{
			name:      "not found",
			status:    404,
			body:      `{"error":"not found"}`,
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
			result, err := artifact.GetArtifact(context.Background(), client, "any-id")

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

func TestUpdateArtifact(t *testing.T) {
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
			body:      `{"id":"xyz-789","metadata":{"name":"Updated"}}`,
			expectErr: false,
			expectID:  "xyz-789",
		},
		{
			name:      "failure",
			status:    422,
			body:      `{"error":"invalid input"}`,
			expectErr: true,
		},
	}

	input := artifact.Artifact{
		Metadata: &artifact.Metadata{Name: "Updated"},
		Data:     map[string]interface{}{"bar": "baz"},
		Specs:    map[string]interface{}{"x": "y"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.body})
			result, err := artifact.UpdateArtifact(context.Background(), client, "xyz-789", input)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectID, result.ID)
			}
		})
	}
}

func TestDeleteArtifact(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		expectErr bool
	}{
		{
			name:      "success",
			status:    204,
			expectErr: false,
		},
		{
			name:      "failure",
			status:    400,
			body:      `{"error":"bad input"}`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(&mockhttp.MockHTTPResponse{StatusCode: tt.status, Body: tt.body})
			err := artifact.DeleteArtifact(context.Background(), client, "abc-123", "db")

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
