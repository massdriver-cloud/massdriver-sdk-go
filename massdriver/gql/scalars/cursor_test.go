package scalars_test

import (
	"reflect"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
)

// TestNewCursor locks in the zero-value omission rules that keep paginated
// requests off the server's `limit: 0` rejection path: neither field set →
// nil (cursor argument omitted entirely), and each field is carried only when
// non-zero so omitempty can drop the other.
func TestNewCursor(t *testing.T) {
	cases := []struct {
		name  string
		limit int
		after string
		want  *scalars.Cursor
	}{
		{name: "neither set → nil", limit: 0, after: "", want: nil},
		{name: "negative limit, no after → nil", limit: -1, after: "", want: nil},
		{name: "limit only", limit: 10, after: "", want: &scalars.Cursor{Limit: 10}},
		{name: "after only", limit: 0, after: "abc", want: &scalars.Cursor{Next: "abc"}},
		{name: "both set", limit: 25, after: "abc", want: &scalars.Cursor{Limit: 25, Next: "abc"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scalars.NewCursor(tc.limit, tc.after)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NewCursor(%d, %q) = %+v, want %+v", tc.limit, tc.after, got, tc.want)
			}
		})
	}
}
