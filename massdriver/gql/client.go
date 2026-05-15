package gql

import (
	"net/http"
	"runtime"
	"runtime/debug"

	"github.com/Khan/genqlient/graphql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
)

const (
	gqlV2Path     = "/api/v2"
	sdkModulePath = "github.com/massdriver-cloud/massdriver-sdk-go"
)

// roundTripperWithHeaders injects a fixed set of HTTP headers on every
// request that flows through it. Used internally by [NewV2Client] to
// attach Authorization, Content-Type, and User-Agent on outgoing
// GraphQL requests.
type roundTripperWithHeaders struct {
	Headers map[string]string
	Base    http.RoundTripper
}

func (r *roundTripperWithHeaders) RoundTrip(req *http.Request) (*http.Response, error) {
	// http.RoundTripper requires that RoundTrip not modify the caller's
	// request — clone before injecting headers.
	req = req.Clone(req.Context())
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	return r.Base.RoundTrip(req)
}

func NewV2Client(cfg config.Config) graphql.Client {
	baseURL := cfg.URL + gqlV2Path

	transport := &roundTripperWithHeaders{
		Base: http.DefaultTransport,
		Headers: map[string]string{
			"Authorization": cfg.Credentials.AuthHeaderValue,
			"Content-Type":  "application/json",
			"User-Agent":    userAgent(),
		},
	}

	httpClient := &http.Client{Transport: transport}
	return graphql.NewClient(baseURL, httpClient)
}

// userAgent reports the User-Agent string GraphQL requests carry. See
// internal/client.UserAgent for the canonical implementation; this is a
// duplicate to avoid an import cycle (internal/client imports gql).
func userAgent() string {
	version := "(unknown)"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Path == sdkModulePath {
			if info.Main.Version != "" {
				version = info.Main.Version
			}
		} else {
			for _, dep := range info.Deps {
				if dep.Path == sdkModulePath {
					version = dep.Version
					break
				}
			}
		}
	}
	return "massdriver-sdk-go/" + version + " go/" + runtime.Version()
}
