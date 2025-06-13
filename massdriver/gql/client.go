package gql

import (
	"net/http"

	"github.com/Khan/genqlient/graphql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
)

const gqlPath = "/api"

type RoundTripperWithHeaders struct {
	Headers map[string]string
	Base    http.RoundTripper
}

func (r *RoundTripperWithHeaders) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	return r.Base.RoundTrip(req)
}

func NewClient(config *config.Config) graphql.Client {
	baseURL := config.URL + gqlPath

	transport := &RoundTripperWithHeaders{
		Base: http.DefaultTransport,
		Headers: map[string]string{
			"Authorization": config.Credentials.AuthHeaderValue,
			"Content-Type":  "application/json",
		},
	}

	httpClient := &http.Client{Transport: transport}
	return graphql.NewClient(baseURL, httpClient)
}
