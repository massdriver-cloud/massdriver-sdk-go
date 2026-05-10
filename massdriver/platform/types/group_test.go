package types_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// TestPolicyConditions_Roundtrip locks in the wire format the rest of
// the SDK depends on. This file is the single source of truth for the
// Conditions scalar's encoding — both policies.Service and
// resources.Service round-trip through these methods, so changes here
// are wire-breaking for both packages.
func TestPolicyConditions_Roundtrip(t *testing.T) {
	cases := []struct {
		name string
		c    types.PolicyConditions
		wire string
	}{
		{
			name: "nil map → whole-policy wildcard",
			c:    nil,
			wire: `"*"`,
		},
		{
			name: "per-key wildcard via nil slice",
			c:    types.PolicyConditions{"md-project": nil},
			wire: `{"md-project":"*"}`,
		},
		{
			name: "per-key wildcard via empty slice",
			c:    types.PolicyConditions{"md-project": {}},
			// Empty slice marshals identically to nil — both mean "any value of this key."
			wire: `{"md-project":"*"}`,
		},
		{
			name: "closed set per key",
			c:    types.PolicyConditions{"md-environment": {"dev", "staging"}},
			wire: `{"md-environment":["dev","staging"]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.c)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != tc.wire {
				t.Errorf("wire = %s, want %s", got, tc.wire)
			}

			var back types.PolicyConditions
			if err := json.Unmarshal([]byte(tc.wire), &back); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			// nil-slice and empty-slice collapse to the same Go value
			// post-unmarshal — per-key wildcards always come back as nil.
			want := tc.c
			if want != nil {
				want = make(types.PolicyConditions, len(tc.c))
				for k, v := range tc.c {
					if len(v) == 0 {
						want[k] = nil
					} else {
						want[k] = v
					}
				}
			}
			if !reflect.DeepEqual(back, want) {
				t.Errorf("roundtrip = %#v, want %#v", back, want)
			}
		})
	}
}

// Mixed condition: one key wildcard, one key closed-set. Common shape
// in real policies ("any project, but only these environments").
func TestPolicyConditions_Mixed(t *testing.T) {
	c := types.PolicyConditions{
		"md-project":     nil,
		"md-environment": {"prod"},
	}
	got, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Map key order isn't guaranteed; round-trip and inspect.
	var back types.PolicyConditions
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back["md-project"] != nil {
		t.Errorf("md-project = %v, want nil (per-key wildcard)", back["md-project"])
	}
	if !reflect.DeepEqual(back["md-environment"], []string{"prod"}) {
		t.Errorf("md-environment = %v, want [prod]", back["md-environment"])
	}
}

func TestPolicyConditions_UnmarshalMalformed(t *testing.T) {
	cases := []string{
		`not json`,
		`{"key": 123}`,        // value isn't string-or-array
		`{"key": "not-star"}`, // string that isn't "*"
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			var c types.PolicyConditions
			if err := json.Unmarshal([]byte(in), &c); err == nil {
				t.Errorf("expected error for malformed input %q, got nil", in)
			}
		})
	}
}
