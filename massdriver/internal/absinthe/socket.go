// Package absinthe implements a thin client for GraphQL subscriptions over
// Phoenix Channels (v2 framing) as exposed by Elixir/Absinthe servers.
//
// It is intentionally generic — callers supply a GraphQL operation string and
// variables, and receive the raw `data` payload of each subscription:data
// frame. No domain types from the surrounding application leak into this
// package.
//
// This package is internal to the SDK. Domain wrappers under
// massdriver/platform/* expose typed streams on top of it (for example,
// deployments.StreamLogs); external consumers go through those rather than
// constructing subscriptions directly.
package absinthe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	controlTopic      = "__absinthe__:control"
	heartbeatTopic    = "phoenix"
	heartbeatEvent    = "heartbeat"
	heartbeatInterval = 30 * time.Second
	replyTimeout      = 30 * time.Second
	socketPath        = "/api/socket/websocket"
	phoenixVsn        = "2.0.0"
)

// Socket is an open Phoenix Channels connection that has joined Absinthe's
// `__absinthe__:control` topic and can multiplex GraphQL subscriptions over
// a single WebSocket.
type Socket struct {
	conn    *websocket.Conn
	nextRef atomic.Uint64

	joinRef string

	writeMu sync.Mutex // serializes writes; gorilla requires single-writer

	mu          sync.Mutex
	pending     map[string]chan reply  // ref → reply slot
	docRefs     map[string]bool        // refs of `doc` pushes awaiting a reply
	subscribers map[string]*subscriber // subscriptionId (== Phoenix topic) → route
	closed      bool
	closeErr    error

	closeConnOne sync.Once     // gates conn.Close so user-Close is idempotent
	connCloseErr error         //   captured once for the caller
	done         chan struct{} // closed when the read loop exits
}

// ErrSocketClosed is the end error of a subscription whose socket closed
// without a transport error, for example when the server went away.
var ErrSocketClosed = errors.New("absinthe socket closed")

// subscriber routes one subscription's data frames. err is why the route
// ended: nil when the client unsubscribed or the server completed the
// subscription. It is set under Socket.mu before ch is closed.
type subscriber struct {
	ch  chan json.RawMessage
	err error
}

// reply carries a phx_reply payload back to the caller awaiting it.
type reply struct {
	status   string
	response json.RawMessage
	// route is the subscription route registered for an ok reply to a
	// `doc` push.
	route *subscriber
}

// Dial opens a WebSocket to baseURL's Phoenix endpoint, joins
// `__absinthe__:control`, and starts the read/heartbeat loops.
//
// baseURL is the HTTPS API base (e.g. "https://api.example.com"). It's
// converted to wss://example.com/api/socket/websocket?token=<token>&vsn=2.0.0.
// The token is sent as a query parameter because Phoenix's UserSocket
// reads auth from connect/3 params (which originate from the upgrade
// URL's query string). Switching to an Authorization header would
// require the server-side UserSocket to opt into connect_info; until
// that lands, query-string is the only auth path the server accepts.
func Dial(ctx context.Context, baseURL, token string) (*Socket, error) {
	wsURL, err := buildWSURL(baseURL, token)
	if err != nil {
		return nil, err
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("absinthe dial: %w", err)
	}

	s := &Socket{
		conn:        conn,
		pending:     make(map[string]chan reply),
		docRefs:     make(map[string]bool),
		subscribers: make(map[string]*subscriber),
		done:        make(chan struct{}),
	}
	go s.readLoop()

	if joinErr := s.join(ctx); joinErr != nil {
		_ = s.Close()
		return nil, joinErr
	}
	go s.heartbeatLoop()
	return s, nil
}

// Close terminates the WebSocket and aborts any in-flight subscriptions.
// Safe to call multiple times and from multiple goroutines.
func (s *Socket) Close() error {
	s.shutdown(nil)
	s.closeConnOne.Do(func() {
		s.connCloseErr = s.conn.Close()
	})
	return s.connCloseErr
}

// Closed reports whether the socket has shut down. A closed socket takes
// no new subscriptions.
func (s *Socket) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Err returns the error that caused the socket to close, if any. Returns nil
// while the socket is healthy.
func (s *Socket) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeErr
}

func (s *Socket) join(ctx context.Context) error {
	s.joinRef = s.refID()
	resp, err := s.push(ctx, &s.joinRef, s.joinRef, controlTopic, "phx_join", json.RawMessage("{}"))
	if err != nil {
		return fmt.Errorf("absinthe join: %w", err)
	}
	if resp.status != "ok" {
		return fmt.Errorf("absinthe join: status=%s response=%s", resp.status, string(resp.response))
	}
	return nil
}

// push writes a frame and (if ref is non-nil) waits for the matching
// phx_reply. joinRef is the channel's join_ref; for control-topic pushes
// it's the join we did at startup, for heartbeats it's nil.
func (s *Socket) push(ctx context.Context, joinRef *string, ref, topic, event string, payload json.RawMessage) (reply, error) {
	ch := make(chan reply, 1)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return reply{}, fmt.Errorf("absinthe socket closed: %w", s.closeErr)
	}
	s.pending[ref] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pending, ref)
		s.mu.Unlock()
	}()

	if err := s.writeFrame(joinRef, &ref, topic, event, payload); err != nil {
		return reply{}, err
	}

	timer := time.NewTimer(replyTimeout)
	defer timer.Stop()

	select {
	case r, ok := <-ch:
		if !ok {
			return reply{}, s.closedErr()
		}
		return r, nil
	case <-ctx.Done():
		return reply{}, ctx.Err()
	case <-s.done:
		return reply{}, s.closedErr()
	case <-timer.C:
		return reply{}, fmt.Errorf("absinthe push %s: timeout waiting for reply", event)
	}
}

// closedErr is the error a push returns once the socket has shut down.
// s.Err() is nil on a clean server-initiated close (isNormalClose
// suppresses it), so that case returns [ErrSocketClosed] alone.
func (s *Socket) closedErr() error {
	if err := s.Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrSocketClosed, err)
	}
	return ErrSocketClosed
}

// writeFrame sends one Phoenix v2 frame. payload may be nil to indicate {}.
func (s *Socket) writeFrame(joinRef, ref *string, topic, event string, payload json.RawMessage) error {
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}
	frame := []any{joinRef, ref, topic, event, payload}
	body, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, body)
}

func (s *Socket) refID() string {
	return strconv.FormatUint(s.nextRef.Add(1), 10)
}

func (s *Socket) readLoop() {
	defer close(s.done)
	for {
		_, body, err := s.conn.ReadMessage()
		if err != nil {
			s.shutdown(err)
			return
		}
		s.dispatch(body)
	}
}

func (s *Socket) dispatch(body []byte) {
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err != nil || len(arr) != 5 {
		// Malformed — ignore. Phoenix doesn't send anything else over WS.
		return
	}
	var (
		joinRef *string
		ref     *string
		topic   string
		event   string
	)
	_ = json.Unmarshal(arr[0], &joinRef)
	_ = json.Unmarshal(arr[1], &ref)
	if err := json.Unmarshal(arr[2], &topic); err != nil {
		return
	}
	if err := json.Unmarshal(arr[3], &event); err != nil {
		return
	}
	payload := arr[4]

	switch event {
	case "phx_reply":
		if ref == nil {
			return
		}
		var p struct {
			Status   string          `json:"status"`
			Response json.RawMessage `json:"response"`
		}
		_ = json.Unmarshal(payload, &p)
		r := reply{status: p.Status, response: p.Response}
		s.mu.Lock()
		ch, ok := s.pending[*ref]
		if s.docRefs[*ref] {
			delete(s.docRefs, *ref)
			// Register the route here, in the read loop, so a frame for the
			// new subscription that follows this reply is not dropped.
			if id := subscriptionID(p.Status, p.Response); id != "" && ok && !s.closed {
				r.route = &subscriber{ch: make(chan json.RawMessage, 64)}
				s.subscribers[id] = r.route
			}
		}
		s.mu.Unlock()
		if ok {
			ch <- r
		}
	case "subscription:data":
		// Payload is {"result": {"data": ..., "errors": ...}, "subscriptionId": "..."}.
		// We pass the whole payload through; the domain caller will pick what it wants.
		// The send happens under mu so a concurrent end cannot close ch first.
		s.mu.Lock()
		if sub, ok := s.subscribers[topic]; ok {
			select {
			case sub.ch <- payload:
			default:
				// drop on slow consumer; subscriptions:data is fire-and-forget
			}
		}
		s.mu.Unlock()
	case "phx_close":
		if topic == controlTopic {
			s.shutdown(fmt.Errorf("absinthe socket received %s on topic %s: %s", event, topic, string(payload)))
			return
		}
		s.endSubscription(topic, nil)
	case "phx_error":
		err := fmt.Errorf("absinthe socket received %s on topic %s: %s", event, topic, string(payload))
		if topic == controlTopic {
			s.shutdown(err)
			return
		}
		s.endSubscription(topic, err)
	}
}

func (s *Socket) heartbeatLoop() {
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
			ref := s.refID()
			// Heartbeats use a nil join_ref. Don't wait for the reply.
			_ = s.writeFrame(nil, &ref, heartbeatTopic, heartbeatEvent, nil)
		}
	}
}

func (s *Socket) shutdown(cause error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if cause != nil && !isNormalClose(cause) {
		s.closeErr = cause
	}
	pending := s.pending
	s.pending = nil
	s.docRefs = nil
	subErr := s.closeErr
	if subErr == nil {
		subErr = ErrSocketClosed
	}
	for _, sub := range s.subscribers {
		sub.err = subErr
		close(sub.ch)
	}
	s.subscribers = nil
	s.mu.Unlock()

	for _, ch := range pending {
		close(ch)
	}
}

// subscriptionID reads the subscriptionId of an ok reply to a `doc` push.
// Returns "" for any other reply.
func subscriptionID(status string, response json.RawMessage) string {
	if status != "ok" {
		return ""
	}
	var r struct {
		SubscriptionID string `json:"subscriptionId"`
	}
	if err := json.Unmarshal(response, &r); err != nil {
		return ""
	}
	return r.SubscriptionID
}

// endSubscription removes topic's route and closes its channel with err as
// the end error. It does nothing when the route is already gone.
func (s *Socket) endSubscription(topic string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subscribers[topic]
	if !ok {
		return
	}
	delete(s.subscribers, topic)
	sub.err = err
	close(sub.ch)
}

// subscriptionErr reads sub's end error.
func (s *Socket) subscriptionErr(sub *subscriber) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sub.err
}

func buildWSURL(baseURL, token string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + socketPath
	q := u.Query()
	q.Set("token", token)
	q.Set("vsn", phoenixVsn)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func isNormalClose(err error) bool {
	if errors.Is(err, websocket.ErrCloseSent) {
		return true
	}
	var ce *websocket.CloseError
	if errors.As(err, &ce) {
		switch ce.Code {
		case websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived:
			return true
		}
	}
	return false
}
