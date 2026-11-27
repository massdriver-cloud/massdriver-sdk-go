package client_test

import (
	"errors"
	"testing"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/absinthe/absinthetest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/streaming"
)

const query = `subscription { alphaStashEvents(organizationId: "my-org", projectId: "api") { key } }`

func newClient(url string, method config.AuthMethod) *client.Client {
	return &client.Client{Config: config.Config{
		OrganizationID: "my-org",
		URL:            url,
		Credentials:    config.Credentials{Method: method, Secret: "mds_test"},
	}}
}

func TestSubscribe_RequiresPAT(t *testing.T) {
	srv := absinthetest.NewServer(t)
	_, _, err := newClient(srv.URL, config.AuthAPIKey).Subscribe(t.Context(), query, nil)
	if !errors.Is(err, streaming.ErrRequiresPAT) {
		t.Fatalf("err = %v, want streaming.ErrRequiresPAT", err)
	}
	if n := srv.Dials(); n != 0 {
		t.Errorf("Dials = %d, want 0", n)
	}
}

func TestSubscribe_NeverReusesDeadSocket(t *testing.T) {
	srv := absinthetest.NewServer(t)
	c := newClient(srv.URL, config.AuthPAT)

	held, releaseHeld, err := c.Subscribe(t.Context(), query, nil)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer releaseHeld()
	srv.WaitForSubscriptions(1)

	srv.Kill()
	select {
	case _, ok := <-held.Data:
		if ok {
			t.Fatal("got data, want the subscription to end")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the subscription to end")
	}
	if held.Err() == nil {
		t.Error("held.Err = nil, want the socket's error")
	}

	_, release, err := c.Subscribe(t.Context(), query, nil)
	if err != nil {
		t.Fatalf("Subscribe after the socket died: %v", err)
	}
	defer release()
	srv.WaitForSubscriptions(2)
	if n := srv.Dials(); n != 2 {
		t.Errorf("Dials = %d, want a new socket while the dead one is still held", n)
	}
}

func TestSubscribe_LastReleaseClosesSocket(t *testing.T) {
	srv := absinthetest.NewServer(t)
	c := newClient(srv.URL, config.AuthPAT)

	first, releaseFirst, err := c.Subscribe(t.Context(), query, nil)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	second, releaseSecond, err := c.Subscribe(t.Context(), query, nil)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	srv.WaitForOpenConns(1)

	_ = first.Close()
	releaseFirst()
	releaseFirst()
	srv.WaitForOpenConns(1)

	_ = second.Close()
	releaseSecond()
	srv.WaitForOpenConns(0)
}
