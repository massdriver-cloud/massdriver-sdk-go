package resources_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/mockhttp"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/provisioning/resources"
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

// Error responses must surface the server's JSON body so 422s are debuggable.
func TestCreateResource_SurfacesServerError(t *testing.T) {
	client, _ := newTestClient(&mockhttp.MockHTTPResponse{
		StatusCode: 422,
		Body:       `{"errors":{"payload":["can't be blank"]}}`,
	})
	service := resources.NewService(client)

	_, err := service.CreateResource(context.Background(), &resources.ResourceInput{Field: "network"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "payload")
	require.Contains(t, err.Error(), "can't be blank")
	require.True(t, strings.Contains(err.Error(), "422"), "expected status in error, got: %s", err.Error())
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
			sentBody:     `{"name":"Created","field":"database","payload":{"foo":"bar","key":"value"}}`,
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

	input := resources.ResourceInput{
		Name:    "Created",
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
			sentBody:     `{"name":"Updated","field":"database","payload":{"bar":"baz","x":"y"}}`,
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

	input := resources.ResourceInput{
		Name:    "Updated",
		Field:   "database",
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
		responseBody string
		expectErr    bool
	}{
		{
			name:      "success",
			status:    200,
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

			err := service.DeleteResource(context.Background(), "abc-123")

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Nil(t, roundTripper.ReceivedRequest.Body)
			}
		})
	}
}

// The provisioned-artifact response carries the resolved resource type, the
// range the producing bundle declared, and the newest version in that range.
// All three must decode.
func TestGetResource_DecodesResourceTypeAndVersionConstraint(t *testing.T) {
	client, _ := newTestClient(&mockhttp.MockHTTPResponse{
		StatusCode: 200,
		Body: `{"id":"my-app.bucket","name":"Bucket","type":"acme/foo",` +
			`"resource_type":"foo@1.2.0","version_constraint":"~1",` +
			`"available_upgrade":"1.3.0","field":"bucket",` +
			`"payload":{"specs":{"zone":"us-east-1"}},"specs":{"zone":"us-east-1"},` +
			`"created_at":"2026-09-01T12:00:00Z","updated_at":"2026-09-02T12:00:00Z"}`,
	})
	service := resources.NewService(client)

	got, err := service.GetResource(context.Background(), "my-app.bucket")
	require.NoError(t, err)
	require.Equal(t, "foo@1.2.0", got.ResourceType)
	require.Equal(t, "~1", got.VersionConstraint)
	// A bare version, unlike ResourceType's `identifier@version`.
	require.Equal(t, "1.3.0", got.AvailableUpgrade)
	// Type stays the legacy org-scoped string, with no version.
	require.Equal(t, "acme/foo", got.Type)
	require.Equal(t, "2026-09-01T12:00:00Z", got.CreatedAt.UTC().Format(time.RFC3339))
	require.Equal(t, "2026-09-02T12:00:00Z", got.UpdatedAt.UTC().Format(time.RFC3339))
	// `specs` is not modeled; it duplicates payload.specs.
	require.Equal(t, map[string]interface{}{"zone": "us-east-1"}, got.Payload["specs"])
}

// An imported resource declares no range, so version_constraint is absent.
func TestGetResource_AbsentVersionConstraint(t *testing.T) {
	client, _ := newTestClient(&mockhttp.MockHTTPResponse{
		StatusCode: 200,
		Body:       `{"id":"imported","name":"Imported","resource_type":"foo@0.0.0","payload":{}}`,
	})
	service := resources.NewService(client)

	got, err := service.GetResource(context.Background(), "imported")
	require.NoError(t, err)
	require.Equal(t, "foo@0.0.0", got.ResourceType)
	require.Empty(t, got.VersionConstraint)
}

// available_upgrade is null once the resource sits on the newest version its
// range accepts — the common case, so the null must decode to empty.
func TestGetResource_NullAvailableUpgrade(t *testing.T) {
	client, _ := newTestClient(&mockhttp.MockHTTPResponse{
		StatusCode: 200,
		Body: `{"id":"my-app.bucket","name":"Bucket","resource_type":"foo@1.3.0",` +
			`"version_constraint":"~1","available_upgrade":null,"payload":{}}`,
	})
	service := resources.NewService(client)

	got, err := service.GetResource(context.Background(), "my-app.bucket")
	require.NoError(t, err)
	require.Equal(t, "~1", got.VersionConstraint)
	require.Empty(t, got.AvailableUpgrade)
}
