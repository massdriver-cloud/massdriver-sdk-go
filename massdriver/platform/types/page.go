package types

import "iter"

// Page is a single page of a paginated list result.
//
// Next and Previous are opaque cursor tokens minted by the server. To advance,
// pass Next as the After field of the next request's input; an empty Next means
// this is the last page. Cursors are not meant to be parsed or persisted across
// schema changes — treat them as opaque handles.
//
// Note: when a list request includes a full-text search, the server returns
// offset-based cursors that are not interchangeable with cursors from a
// non-search request on the same endpoint. Don't mix them.
type Page[T any] struct {
	// Items are the records on this page, in server-defined sort order.
	Items []T
	// Next is the cursor for the following page, or "" if this is the last page.
	Next string
	// Previous is the cursor for the preceding page, or "" if this is the first.
	Previous string
}

// Collect drains a paginated iterator into a single slice, stopping at the
// first error. It is the [iter.Seq2] analogue of [slices.Collect], which cannot
// consume the (value, error) iterators the SDK's Iter methods return.
//
// Use it when you genuinely want every match in memory and the result set is
// bounded. For large or unbounded sets, range the iterator directly so you can
// process items as they stream and stop early:
//
//	for p, err := range client.Projects.Iter(ctx, projects.ListInput{}) {
//	    if err != nil { return err }
//	    ...
//	}
//
// On error Collect returns a nil slice (never a partial page) so a truncated
// result is never mistaken for a complete one.
func Collect[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var out []T
	for v, err := range seq {
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
