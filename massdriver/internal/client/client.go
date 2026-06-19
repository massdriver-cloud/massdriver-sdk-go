package client

import (
	"runtime"
	"runtime/debug"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
)

// DefaultTimeout is the per-request HTTP timeout applied when callers
// don't override it. Long enough for typical reads and writes; short
// enough that a hung backend can't pin a goroutine forever. Streaming
// operations bypass this — they use long-lived connections gated by
// the caller's context.
const DefaultTimeout = 30 * time.Second

// sdkModulePath identifies this SDK to UserAgent. Must match the go.mod
// module path so [debug.ReadBuildInfo] can find our version when the
// SDK is consumed as a dependency.
const sdkModulePath = "github.com/massdriver-cloud/massdriver-sdk-go"

// UserAgent reports the User-Agent string outbound requests carry. The
// SDK version is sourced from [debug.ReadBuildInfo] — when the SDK is
// imported as a tagged module dependency it resolves to e.g. "v0.3.1";
// in local development builds it resolves to "(devel)".
func UserAgent() string {
	version := "(unknown)"
	if info, ok := debug.ReadBuildInfo(); ok {
		// Self-test path: building inside this module.
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

// Client is the low-level transport bag used by every domain Service.
// It is intentionally hidden from external callers — the public entry
// point is the top-level [massdriver.Client] which embeds an instance
// of this type.
type Client struct {
	Config config.Config
	HTTP   *resty.Client
	GQLv2  graphql.Client
}

// New constructs a [*Client] from environment variables and the
// active profile in ~/.config/massdriver/config.yaml.
//
// Equivalent to NewWithConfig with the default flow when callers
// pass no options.
func New() (*Client, error) {
	cfg, cfgErr := config.Get()
	if cfgErr != nil {
		return nil, cfgErr
	}
	return NewWithConfig(cfg, DefaultTimeout), nil
}

// NewWithConfig constructs a [*Client] from a fully-resolved [config.Config]
// and a per-request HTTP timeout. Used by the top-level
// [massdriver.NewClient] after applying functional options, and by tests
// that need explicit control over the configured values.
//
// A timeout of 0 disables the per-request HTTP timeout — only do this
// for callers who already enforce deadlines via [context.Context].
func NewWithConfig(cfg config.Config, timeout time.Duration) *Client {
	rest := resty.New().
		SetBaseURL(cfg.URL).
		SetTimeout(timeout).
		SetHeader("Authorization", cfg.Credentials.AuthHeaderValue).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("User-Agent", UserAgent())

	return &Client{
		Config: cfg,
		HTTP:   rest,
		GQLv2:  gql.NewV2Client(cfg),
	}
}
