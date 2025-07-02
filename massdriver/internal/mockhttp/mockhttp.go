package mockhttp

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

type MockHTTPResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

type MockTransport struct {
	Responses       map[string]*MockHTTPResponse // key: "METHOD URL"
	ReceivedRequest *http.Request
	CallCount       int
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		Responses: make(map[string]*MockHTTPResponse),
	}
}

func (m *MockTransport) RegisterResponse(method, url string, response *MockHTTPResponse) {
	key := fmt.Sprintf("%s %s", method, url)
	m.Responses[key] = response
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.ReceivedRequest = req
	m.CallCount++

	key := fmt.Sprintf("%s %s", req.Method, req.URL.String())
	mockResponse, exists := m.Responses[key]
	if !exists {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error": "mock not found"}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    req,
		}, nil
	}

	headers := http.Header{"Content-Type": []string{"application/json"}}
	if mockResponse.Headers != nil {
		for k, v := range mockResponse.Headers {
			headers.Set(k, v)
		}
	}

	return &http.Response{
		StatusCode: mockResponse.StatusCode,
		Body:       io.NopCloser(bytes.NewBufferString(mockResponse.Body)),
		Header:     headers,
		Request:    req,
	}, nil
}
