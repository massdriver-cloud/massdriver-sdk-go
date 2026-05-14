package types_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// TestPolicyConditions_Roundtrip locks in the plain user-facing JSON
// form. The GraphQL wire form is a separate concern handled by
// gql/scalars; application code that calls json.Marshal on a
// PolicyConditions value gets the form tested here.
func TestPolicyConditions_Roundtrip(t *testing.T) {
	cases := []struct {
		name string
		c    types.PolicyConditions
		json string
	}{
		{
			name: "nil map → null",
			c:    nil,
			json: `null`,
		},
		{
			name: "per-key wildcard via nil slice",
			c:    types.PolicyConditions{"md-project": nil},
			json: `{"md-project":"*"}`,
		},
		{
			name: "per-key wildcard via empty slice",
			c:    types.PolicyConditions{"md-project": {}},
			json: `{"md-project":"*"}`,
		},
		{
			name: "closed set per key",
			c:    types.PolicyConditions{"md-environment": {"dev", "staging"}},
			json: `{"md-environment":["dev","staging"]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.c)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != tc.json {
				t.Errorf("json = %s, want %s", got, tc.json)
			}

			var back types.PolicyConditions
			if err := json.Unmarshal([]byte(tc.json), &back); err != nil {
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

// Mixed condition: one key wildcard, one key closed-set.
func TestPolicyConditions_Mixed(t *testing.T) {
	c := types.PolicyConditions{
		"md-project":     nil,
		"md-environment": {"prod"},
	}
	got, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
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
