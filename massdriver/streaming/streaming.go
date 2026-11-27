// Package streaming holds primitives shared by every Massdriver SDK
// streaming subscription.
//
// The Service-level streaming methods live in their per-domain packages
// ([deployments.Service.StreamLogs], [instances.Service.StreamEvents],
// etc.) and return a [Stream]. This package holds [Stream] and the
// sentinels callers classify errors with [errors.Is].
//
// The streams of one client share a single WebSocket. It opens with the
// first stream and closes when the last one ends.
package streaming

import "errors"

// ErrRequiresPAT is returned by every Stream* operation when the
// configured credentials are not a personal access token. WebSocket
// subscriptions authenticate via a query-string token that only works
// for PATs — basic-auth API keys are rejected by the server's
// UserSocket.
//
// Match with [errors.Is]:
//
//	_, err := c.Instances.StreamEvents(ctx, id)
//	if errors.Is(err, streaming.ErrRequiresPAT) {
//	    // surface a hint to set MASSDRIVER_API_KEY to a mds_/md_ token
//	}
var ErrRequiresPAT = errors.New(
	"streaming requires a personal access token (set MASSDRIVER_API_KEY to a token starting with mds_/md_)",
)
