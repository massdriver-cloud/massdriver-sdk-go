package gql_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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
