package gql_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/stretchr/testify/require"
)

func TestRoundTripperWithHeaders_SetsHeaders(t *testing.T) {
	var capturedRequest *http.Request

	// Fake server that captures the incoming request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"data": {}}`)
	}))
	defer server.Close()

	// Custom RoundTripper that wraps default transport
	rt := &gql.RoundTripperWithHeaders{
		Base: http.DefaultTransport,
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
			"Content-Type":  "application/json",
		},
	}

	httpClient := &http.Client{Transport: rt}
	req, err := http.NewRequestWithContext(context.TODO(), "POST", server.URL, nil)
	require.NoError(t, err)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()

	require.NotNil(t, capturedRequest)
	require.Equal(t, "Bearer test-token", capturedRequest.Header.Get("Authorization"))
	require.Equal(t, "application/json", capturedRequest.Header.Get("Content-Type"))
}

func TestNewV2Client_UsesV2Path(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"data": {}}`)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL: server.URL,
		Credentials: &config.Credentials{
			AuthHeaderValue: "Bearer test-token",
		},
	}

	client := gql.NewV2Client(cfg)

	var resp graphql.Response
	err := client.MakeRequest(context.TODO(), &graphql.Request{Query: "query { __typename }"}, &resp)
	require.NoError(t, err)
	require.Equal(t, "/api/v2", capturedPath)
}
