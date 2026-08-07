// Package filters builds genqlient filter inputs from SDK-level values.
// It holds the mappings shared by the platform wrapper packages so each
// list endpoint doesn't reimplement them.
package filters

import (
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Datetime maps an inclusive [after, before] window onto the generated
// input, leaving zero bounds unset so open bounds stay off the wire.
// Callers guard the both-zero case themselves (omit the filter entirely).
func Datetime(after, before time.Time) *gen.DatetimeFilter {
	dt := &gen.DatetimeFilter{}
	if !after.IsZero() {
		dt.Gte = &after
	}
	if !before.IsZero() {
		dt.Lte = &before
	}
	return dt
}

// Attributes maps the SDK's attribute filters onto the generated input
// type.
func Attributes(in []types.AttributeFilter) []gen.AttributeFilter {
	out := make([]gen.AttributeFilter, 0, len(in))
	for _, a := range in {
		out = append(out, gen.AttributeFilter{Key: a.Key, Eq: a.Eq, In: a.In})
	}
	return out
}
