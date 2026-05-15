package client

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/absinthe"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/streaming"
)

// OpenStreamSocket gates streaming on PAT auth and opens an Absinthe
// socket bound to the client's configured base URL and token. Used by
// every Service.Stream* method so the auth check and dial sequence live
// in one place.
//
// Returns [streaming.ErrRequiresPAT] before any network I/O when the
// client is configured with basic-auth credentials.
func (c *Client) OpenStreamSocket(ctx context.Context) (*absinthe.Socket, error) {
	if c.Config.Credentials.Method != config.AuthPAT {
		return nil, streaming.ErrRequiresPAT
	}
	socket, err := absinthe.Dial(ctx, c.Config.URL, c.Config.Credentials.Secret)
	if err != nil {
		return nil, fmt.Errorf("open absinthe socket: %w", err)
	}
	return socket, nil
}
