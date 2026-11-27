package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/absinthe"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/streaming"
)

// streamSockets holds the one Absinthe socket that a client's
// subscriptions share. The zero value is ready to use.
type streamSockets struct {
	mu      sync.Mutex
	current *sharedSocket
}

// sharedSocket is a socket and the number of subscriptions that use it.
// users is guarded by streamSockets.mu.
type sharedSocket struct {
	socket *absinthe.Socket
	users  int
}

// Subscribe opens a GraphQL subscription on the client's shared Absinthe
// socket. The socket opens with the first subscription. A socket that has
// died is never reused: the next subscription opens a new one.
//
// The caller must call release once the subscription has ended or been
// closed. The socket closes when its last user releases it.
//
// Returns [streaming.ErrRequiresPAT] before any network I/O when the
// client is configured with basic-auth credentials.
func (c *Client) Subscribe(ctx context.Context, query string, vars map[string]any) (sub *absinthe.Subscription, release func(), err error) {
	if c.Config.Credentials.Method != config.AuthPAT {
		return nil, nil, streaming.ErrRequiresPAT
	}
	shared, err := c.acquireSocket(ctx)
	if err != nil {
		return nil, nil, err
	}
	var once sync.Once
	release = func() { once.Do(func() { c.releaseSocket(shared) }) }

	sub, err = shared.socket.Subscribe(ctx, query, vars)
	if err != nil {
		release()
		return nil, nil, err
	}
	return sub, release, nil
}

func (c *Client) acquireSocket(ctx context.Context) (*sharedSocket, error) {
	c.sockets.mu.Lock()
	defer c.sockets.mu.Unlock()
	if c.sockets.current == nil || c.sockets.current.socket.Closed() {
		socket, err := absinthe.Dial(ctx, c.Config.URL, c.Config.Credentials.Secret)
		if err != nil {
			return nil, fmt.Errorf("open absinthe socket: %w", err)
		}
		c.sockets.current = &sharedSocket{socket: socket}
	}
	c.sockets.current.users++
	return c.sockets.current, nil
}

func (c *Client) releaseSocket(shared *sharedSocket) {
	c.sockets.mu.Lock()
	defer c.sockets.mu.Unlock()
	shared.users--
	if shared.users > 0 {
		return
	}
	_ = shared.socket.Close()
	if c.sockets.current == shared {
		c.sockets.current = nil
	}
}
