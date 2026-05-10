// Package gen holds the genqlient-generated bindings for Massdriver's V2
// GraphQL API. It sits under massdriver/internal/ deliberately — external
// consumers go through the typed wrappers in massdriver/platform/* rather
// than constructing queries directly, so the generated types are not part
// of the public API surface and may change at any time.
package gen

//go:generate go run github.com/Khan/genqlient
