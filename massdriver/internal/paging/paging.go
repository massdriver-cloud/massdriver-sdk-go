// Package paging holds the shared cursor-driven iteration machinery behind the
// platform services' Iter methods. Each service supplies a FetchFunc that knows
// how to retrieve one page for its entity; this package turns that into a lazy
// [iter.Seq2] that walks every page on demand.
package paging

import (
	"context"
	"iter"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// FetchFunc retrieves a single page for an entity, given an opaque "after"
// cursor ("" selects the first page). It returns the page (whose Next drives
// further iteration) or an error.
type FetchFunc[T any] func(ctx context.Context, after string) (types.Page[T], error)

// Iter returns a lazy iterator over every item matching a request, fetching
// pages on demand via fetch starting from the after cursor. The yielded error
// is non-nil exactly once, when a page fetch fails, after which iteration
// stops. Breaking out of the range loop stops requesting further pages.
func Iter[T any](ctx context.Context, after string, fetch FetchFunc[T]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		cursor := after
		for {
			page, err := fetch(ctx, cursor)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			if page.Next == "" {
				return
			}
			cursor = page.Next
		}
	}
}
