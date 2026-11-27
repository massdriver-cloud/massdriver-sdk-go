package stash_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/stash"
)

func TestUnpackEvent(t *testing.T) {
	t.Run("set JSON entry", func(t *testing.T) {
		ev, ok := stash.UnpackEvent([]byte(`{"alphaStashEvents": {
			"action": "SET",
			"scope": {"kind": "INSTANCE", "id": "api-prod-db"},
			"key": "md/v1/usage",
			"entry": {
				"scope": {"kind": "INSTANCE", "id": "api-prod-db"},
				"key": "md/v1/usage",
				"label": "Disk",
				"value": "{\"value\":42,\"max\":100}",
				"format": "JSON",
				"immutable": false,
				"sensitive": false,
				"expiresAt": null,
				"updatedAt": "2026-11-20T10:00:00Z"
			},
			"timestamp": "2026-11-20T10:00:00Z"
		}}`))
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if ev.Action != stash.EventSet || ev.Key != "md/v1/usage" || ev.Timestamp.IsZero() {
			t.Errorf("event = %+v", ev)
		}
		if ev.Scope != (stash.Scope{Kind: stash.ScopeInstance, ID: "api-prod-db"}) {
			t.Errorf("Scope = %+v", ev.Scope)
		}
		if ev.Entry == nil || ev.Entry.Scope.Kind != stash.ScopeInstance || ev.Entry.Label != "Disk" {
			t.Fatalf("Entry = %+v", ev.Entry)
		}
		if ev.Entry.Data["value"] != float64(42) || ev.Entry.Data["max"] != float64(100) {
			t.Errorf("Data = %v", ev.Entry.Data)
		}
	})

	t.Run("sensitive entry has no value", func(t *testing.T) {
		ev, ok := stash.UnpackEvent([]byte(`{"alphaStashEvents": {
			"action": "SET",
			"scope": {"kind": "PROJECT", "id": "api"},
			"key": "password",
			"entry": {"scope": {"kind": "PROJECT", "id": "api"}, "key": "password", "value": null, "format": "TEXT", "sensitive": true, "updatedAt": "2026-11-20T10:00:00Z"},
			"timestamp": "2026-11-20T10:00:00Z"
		}}`))
		if !ok || ev.Entry == nil || ev.Entry.Value != nil || !ev.Entry.Sensitive {
			t.Errorf("ok = %v, event = %+v", ok, ev)
		}
	})

	t.Run("deleted", func(t *testing.T) {
		ev, ok := stash.UnpackEvent([]byte(`{"alphaStashEvents": {
			"action": "DELETED",
			"scope": {"kind": "DEPLOYMENT", "id": "a1b2c3d4-0000-0000-0000-000000000001"},
			"key": "migrations/ran",
			"entry": null,
			"timestamp": "2026-11-20T10:00:00Z"
		}}`))
		if !ok || ev.Action != stash.EventDeleted || ev.Entry != nil || ev.Scope.Kind != stash.ScopeDeployment {
			t.Errorf("ok = %v, event = %+v", ok, ev)
		}
	})

	t.Run("skips malformed frames", func(t *testing.T) {
		for _, raw := range []string{`not json`, `{}`, `{"alphaStashEvents": null}`} {
			if _, ok := stash.UnpackEvent([]byte(raw)); ok {
				t.Errorf("UnpackEvent(%s) ok = true, want false", raw)
			}
		}
	})
}
