package scalars_test

import (
	"reflect"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// TestMarshalConditions locks in the GraphQL wire form: a JSON-encoded
// string whose contents are either `"*"` (whole-policy wildcard) or a
// JSON object literal.
func TestMarshalConditions(t *testing.T) {
	cases := []struct {
		name string
		in   types.PolicyConditions
		wire string
	}{
		{name: "nil → whole-policy wildcard", in: nil, wire: `"*"`},
		{name: "per-key wildcard", in: types.PolicyConditions{"team": nil}, wire: `"{\"team\":\"*\"}"`},
		{name: "closed set", in: types.PolicyConditions{"env": {"dev", "stg"}}, wire: `"{\"env\":[\"dev\",\"stg\"]}"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := scalars.MarshalConditions(&tc.in)
			if err != nil {
				t.Fatalf("MarshalConditions: %v", err)
			}
			if string(got) != tc.wire {
				t.Errorf("wire = %s, want %s", got, tc.wire)
			}
		})
	}
}

// TestUnmarshalConditions accepts both the input-side wire form
// (JSON-encoded string) and the response-side bare-object form, since
// the platform is asymmetric.
func TestUnmarshalConditions(t *testing.T) {
	cases := []struct {
		name string
		wire string
		want types.PolicyConditions
	}{
		{name: "wire wildcard", wire: `"*"`, want: nil},
		{name: "wire object", wire: `"{\"team\":[\"eng\"]}"`, want: types.PolicyConditions{"team": {"eng"}}},
		{name: "bare object response", wire: `{"team":["eng"]}`, want: types.PolicyConditions{"team": {"eng"}}},
		{name: "bare per-key wildcard", wire: `{"team":"*"}`, want: types.PolicyConditions{"team": nil}},
		{name: "null", wire: `null`, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got types.PolicyConditions
			if err := scalars.UnmarshalConditions([]byte(tc.wire), &got); err != nil {
				t.Fatalf("UnmarshalConditions: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
