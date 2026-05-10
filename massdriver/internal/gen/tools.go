//go:build tools

package gen

import (
	// Keeps genqlient's CLI dependencies (alexflint/go-arg, alexflint/go-scalar,
	// agnivade/levenshtein, etc.) from being pruned by `go mod tidy`.
	_ "github.com/Khan/genqlient/generate"
)
