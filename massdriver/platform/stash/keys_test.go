package stash_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/stash"
)

func TestJSONInput(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"usage", stash.Usage{Value: 42, Max: 100, Label: "Disk"}, `{"value":42,"max":100,"label":"Disk"}`},
		{"usage without label", stash.Usage{Value: 0, Max: 1}, `{"value":0,"max":1}`},
		{"traffic", stash.Traffic{RPS: 12.5, Errors5m: 3, P95ms: 180}, `{"rps":12.5,"errors_5m":3,"p95_ms":180}`},
		{"traffic without latency", stash.Traffic{RPS: 0, Errors5m: 0}, `{"rps":0,"errors_5m":0}`},
		{"banner", stash.Banner{Message: "Freeze until Monday", Level: stash.BannerWarning}, `{"message":"Freeze until Monday","level":"warning"}`},
		{"links", stash.Links{Links: []stash.Link{{Label: "Dashboard", URL: "https://grafana.example.com/d/api"}}}, `{"links":[{"label":"Dashboard","url":"https://grafana.example.com/d/api"}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stash.JSONInput(tc.value)
			if err != nil {
				t.Fatalf("JSONInput: %v", err)
			}
			if got.Value != tc.want || got.Format != stash.FormatJSON {
				t.Errorf("got %+v, want Value %s and Format JSON", got, tc.want)
			}
		})
	}
}

func TestJSONInput_NotAnObject(t *testing.T) {
	for _, v := range []any{"healthy", []int{1}, nil, func() {}} {
		if _, err := stash.JSONInput(v); err == nil {
			t.Errorf("JSONInput(%T) err = nil, want an error", v)
		}
	}
}
