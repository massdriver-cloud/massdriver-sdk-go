// Package paging holds the shared cursor-driven iteration machinery behind the
// platform services' Iter methods. Each service supplies a FetchFunc that knows
// how to retrieve one page for its entity; this package turns that into a lazy
// [iter.Seq2] that walks every page on demand.
package paging

import (
	"context"
	"fmt"
	"iter"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// FetchFunc retrieves a single page for an entity, given an opaque "after"
// cursor ("" selects the first page). It returns the page (whose Next drives
// further iteration) or an error.
type FetchFunc[T any] func(ctx context.Context, after string) (types.Page[T], error)

// RawPage is one page of a paginated sub-list as it comes off a genqlient
// response: the raw items slice plus the page's cursors. Returned by the
// fetch callbacks passed to [DecodeFetch].
type RawPage struct {
	// Items is the genqlient-generated items slice, decoded into the public
	// type by [DecodeFetch].
	Items any
	// Next and Previous are the page's opaque pagination cursors.
	Next     string
	Previous string
}

// DecodeFetch builds a [FetchFunc] for a paginated sub-list from the query
// call that fetches one raw page. It folds the limit into the cursor,
// decodes each page's genqlient items into the public type T, and shapes the
// result into a [types.Page] — the boilerplate otherwise repeated by every
// sub-list fetcher. label names the item in decode error messages.
func DecodeFetch[T any](limit int, label string, fetch func(ctx context.Context, cursor *scalars.Cursor) (RawPage, error)) FetchFunc[T] {
	return func(ctx context.Context, after string) (types.Page[T], error) {
		raw, err := fetch(ctx, scalars.NewCursor(limit, after))
		if err != nil {
			return types.Page[T]{}, err
		}
		items := []T{}
		if derr := decode.Decode(raw.Items, &items); derr != nil {
			return types.Page[T]{}, fmt.Errorf("decode %s page: %w", label, derr)
		}
		return types.Page[T]{Items: items, Next: raw.Next, Previous: raw.Previous}, nil
	}
}

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
