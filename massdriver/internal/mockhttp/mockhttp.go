package mockhttp

import (
	"bytes"
	"io"
	"net/http"
)

type MockHTTPResponse struct {
	StatusCode int
	Body       string
}

type MutableRoundTripper struct {
	Response *MockHTTPResponse
}

func (m *MutableRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.Response.StatusCode,
		Body:       io.NopCloser(bytes.NewBufferString(m.Response.Body)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Request: req,
	}, nil
}
