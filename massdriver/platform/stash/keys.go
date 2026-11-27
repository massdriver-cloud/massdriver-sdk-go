package stash

import (
	"encoding/json"
	"fmt"
)

// Well-known keys drive the widgets in [Service.WidgetCatalog]. The server
// checks a well-known key's value against the widget's schema, and rejects
// one that is sensitive or on the wrong kind of scope.
const (
	// KeyHealthV1 is a TEXT key on an instance. Its value is the health,
	// such as [HealthHealthy].
	KeyHealthV1 = "md/v1/health"
	// KeyUsageV1 is a JSON key on an instance. Its value is a [Usage].
	KeyUsageV1 = "md/v1/usage"
	// KeyTrafficV1 is a JSON key on the instance that sends the traffic.
	// Its value is a [Traffic].
	KeyTrafficV1 = "md/v1/traffic"
	// KeyBannerV1 is a JSON key on an environment. Its value is a [Banner].
	KeyBannerV1 = "md/v1/banner"
	// KeyLinksV1 is a JSON key on an environment. Its value is a [Links].
	KeyLinksV1 = "md/v1/links"
)

// Health values the Massdriver UI has colors for. [KeyHealthV1] takes any
// string up to 64 characters.
const (
	HealthHealthy  = "healthy"
	HealthDegraded = "degraded"
	HealthDown     = "down"
)

// Usage is the value of [KeyUsageV1]: how much of a limit is used. Max
// must be greater than zero.
type Usage struct {
	Value float64 `json:"value"`
	Max   float64 `json:"max"`
	// Label is text shown with the bar, up to 64 characters.
	Label string `json:"label,omitempty"`
}

// Traffic is the value of [KeyTrafficV1]: the traffic on the links that
// leave an instance.
type Traffic struct {
	// RPS is requests per second.
	RPS float64 `json:"rps"`
	// Errors5m is the count of errors in the last five minutes.
	Errors5m int `json:"errors_5m"`
	// P95ms is the 95th percentile response time in milliseconds. Zero
	// leaves it out.
	P95ms float64 `json:"p95_ms,omitempty"`
}

// Banner levels. The UI uses [BannerInfo] when Level is empty.
const (
	BannerInfo    = "info"
	BannerWarning = "warning"
	BannerError   = "error"
)

// Banner is the value of [KeyBannerV1]: a message across the top of an
// environment.
type Banner struct {
	// Message is up to 280 characters.
	Message string `json:"message"`
	// Level is [BannerInfo], [BannerWarning], or [BannerError].
	Level string `json:"level,omitempty"`
}

// Links is the value of [KeyLinksV1]: from 1 to 8 links in the header of
// an environment.
type Links struct {
	Links []Link `json:"links"`
}

// Link is one link in [Links]. Label is up to 40 characters, and URL is an
// http or https URL.
type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// JSONInput returns a [SetInput] whose Value is v marshaled to JSON, with
// [FormatJSON]. Use it with a well-known key's value type, such as [Usage]
// or [Step]. v must marshal to a JSON object.
func JSONInput(v any) (SetInput, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return SetInput{}, fmt.Errorf("marshal stash value: %w", err)
	}
	if len(data) == 0 || data[0] != '{' {
		return SetInput{}, fmt.Errorf("stash JSON value must be an object, got %s", data)
	}
	return SetInput{Value: string(data), Format: FormatJSON}, nil
}
