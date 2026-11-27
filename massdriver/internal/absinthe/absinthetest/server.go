// Package absinthetest runs a fake Phoenix Channels endpoint that speaks
// enough of Absinthe's subscription protocol to test the SDK's streaming
// against: it joins the control topic, accepts subscription documents,
// pushes data frames, completes subscriptions, and drops connections.
package absinthetest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const waitTimeout = 5 * time.Second

// Server is a fake Absinthe endpoint. Its URL is the API base URL to give
// the SDK client.
type Server struct {
	*httptest.Server

	// AfterSubscribe, when set, runs right after the server replies to a
	// subscription document, before it reads the next frame.
	AfterSubscribe func(id string)

	t        testing.TB
	upgrader websocket.Upgrader

	mu           sync.Mutex
	dials        int
	conns        map[*conn]bool
	subs         map[string]*conn // live subscription ID → connection
	order        []string         // subscription IDs in creation order
	unsubscribed []string
}

type conn struct {
	ws      *websocket.Conn
	writeMu sync.Mutex
}

// NewServer starts a Server and stops it when the test ends.
func NewServer(t testing.TB) *Server {
	t.Helper()
	s := &Server{
		t:     t,
		conns: map[*conn]bool{},
		subs:  map[string]*conn{},
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// Dials reports how many WebSocket connections clients have opened.
func (s *Server) Dials() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dials
}

// WaitForSubscriptions waits until n subscriptions have been created over
// the server's life and returns their IDs in creation order.
func (s *Server) WaitForSubscriptions(n int) []string {
	s.t.Helper()
	var ids []string
	s.waitFor(fmt.Sprintf("%d subscriptions", n), func() bool {
		ids = slices.Clone(s.order)
		return len(ids) >= n
	})
	return ids
}

// WaitForUnsubscribe waits until a client unsubscribes id.
func (s *Server) WaitForUnsubscribe(id string) {
	s.t.Helper()
	s.waitFor("unsubscribe of "+id, func() bool { return slices.Contains(s.unsubscribed, id) })
}

// WaitForOpenConns waits until exactly n connections are open.
func (s *Server) WaitForOpenConns(n int) {
	s.t.Helper()
	s.waitFor(fmt.Sprintf("%d open connections", n), func() bool { return len(s.conns) == n })
}

// Push sends data as one subscription:data frame for subscription id.
func (s *Server) Push(id string, data any) {
	s.t.Helper()
	c := s.subConn(id)
	payload := map[string]any{"result": map[string]any{"data": data}, "subscriptionId": id}
	if err := c.write(nil, nil, id, "subscription:data", payload); err != nil {
		s.t.Fatalf("push to %s: %v", id, err)
	}
}

// Complete ends subscription id from the server side with phx_close.
func (s *Server) Complete(id string) {
	s.t.Helper()
	c := s.subConn(id)
	s.mu.Lock()
	delete(s.subs, id)
	s.mu.Unlock()
	if err := c.write(nil, nil, id, "phx_close", map[string]any{}); err != nil {
		s.t.Fatalf("complete %s: %v", id, err)
	}
}

// Fail ends subscription id from the server side with phx_error.
func (s *Server) Fail(id string) {
	s.t.Helper()
	c := s.subConn(id)
	s.mu.Lock()
	delete(s.subs, id)
	s.mu.Unlock()
	if err := c.write(nil, nil, id, "phx_error", map[string]any{"reason": "boom"}); err != nil {
		s.t.Fatalf("fail %s: %v", id, err)
	}
}

// Kill drops every open connection without a close frame, as a network
// failure would.
func (s *Server) Kill() {
	s.mu.Lock()
	conns := make([]*conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.ws.UnderlyingConn().Close()
	}
}

func (s *Server) subConn(id string) *conn {
	s.t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.subs[id]
	if !ok {
		s.t.Fatalf("no live subscription %s", id)
	}
	return c
}

func (s *Server) waitFor(what string, cond func() bool) {
	s.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		s.mu.Lock()
		ok := cond()
		s.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/socket/websocket" || r.URL.Query().Get("token") == "" {
		http.NotFound(w, r)
		return
	}
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &conn{ws: ws}
	s.mu.Lock()
	s.dials++
	s.conns[c] = true
	s.mu.Unlock()

	defer func() {
		_ = ws.Close()
		s.mu.Lock()
		delete(s.conns, c)
		for id, owner := range s.subs {
			if owner == c {
				delete(s.subs, id)
			}
		}
		s.mu.Unlock()
	}()

	for {
		_, body, err := ws.ReadMessage()
		if err != nil {
			return
		}
		s.handle(c, body)
	}
}

func (s *Server) handle(c *conn, body []byte) {
	var frame []json.RawMessage
	if err := json.Unmarshal(body, &frame); err != nil || len(frame) != 5 {
		return
	}
	var joinRef, ref *string
	var topic, event string
	_ = json.Unmarshal(frame[0], &joinRef)
	_ = json.Unmarshal(frame[1], &ref)
	_ = json.Unmarshal(frame[2], &topic)
	_ = json.Unmarshal(frame[3], &event)

	response := map[string]any{}
	switch event {
	case "doc":
		s.mu.Lock()
		id := fmt.Sprintf("__absinthe__:doc:%d", len(s.order)+1)
		s.subs[id] = c
		s.order = append(s.order, id)
		s.mu.Unlock()
		response["subscriptionId"] = id
	case "unsubscribe":
		var p struct {
			SubscriptionID string `json:"subscriptionId"`
		}
		_ = json.Unmarshal(frame[4], &p)
		s.mu.Lock()
		delete(s.subs, p.SubscriptionID)
		s.unsubscribed = append(s.unsubscribed, p.SubscriptionID)
		s.mu.Unlock()
	}
	_ = c.write(joinRef, ref, topic, "phx_reply", map[string]any{"status": "ok", "response": response})
	if id, ok := response["subscriptionId"].(string); ok && s.AfterSubscribe != nil {
		s.AfterSubscribe(id)
	}
}

func (c *conn) write(joinRef, ref *string, topic, event string, payload any) error {
	body, err := json.Marshal([]any{joinRef, ref, topic, event, payload})
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteMessage(websocket.TextMessage, body)
}
