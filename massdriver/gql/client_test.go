package gql_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/stretchr/testify/require"
)

func TestNewV2Client_UsesV2Path(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"data": {}}`)
	}))
	defer server.Close()

	cfg := config.Config{
		URL: server.URL,
		Credentials: config.Credentials{
			AuthHeaderValue: "Bearer test-token",
		},
	}

	client := gql.NewV2Client(cfg)

	var resp graphql.Response
	err := client.MakeRequest(context.TODO(), &graphql.Request{Query: "query { __typename }"}, &resp)
	require.NoError(t, err)
	require.Equal(t, "/api/v2", capturedPath)
}

// TestNewV2Client_SetsUserAgent confirms outbound requests carry a
// User-Agent identifying this SDK — important for ops-side debugging
// and for the platform team to track SDK adoption.
func TestNewV2Client_SetsUserAgent(t *testing.T) {
	var capturedUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"data": {}}`)
	}))
	defer server.Close()

	cfg := config.Config{URL: server.URL, Credentials: config.Credentials{AuthHeaderValue: "x"}}
	client := gql.NewV2Client(cfg)
	_ = client.MakeRequest(context.TODO(), &graphql.Request{Query: "query { __typename }"}, &graphql.Response{})

	if !strings.HasPrefix(capturedUA, "massdriver-sdk-go/") {
		t.Errorf("User-Agent = %q, want prefix massdriver-sdk-go/", capturedUA)
	}
	if !strings.Contains(capturedUA, "go/") {
		t.Errorf("User-Agent = %q, want it to include go/<version>", capturedUA)
	}
}
