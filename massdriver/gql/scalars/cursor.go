package scalars

// Cursor is the GraphQL `Cursor` input type. Defined here (rather than letting
// genqlient generate it) so that the `omitempty` tags drop zero-value fields —
// otherwise paginated requests send `limit: 0`, which the server rejects since
// `Cursor.limit` is constrained to 1..100.
//
// Bound to the GraphQL `Cursor` type via massdriver/internal/gen/genqlient.yaml.
type Cursor struct {
	Limit    int    `json:"limit,omitempty"`
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
}

// NewCursor builds the *Cursor for a paginated request from a page-size limit
// and an opaque "after" token (the Next cursor of a prior page; "" for the
// first page). It returns nil when neither is set, so the request omits the
// cursor argument entirely rather than sending an empty object.
func NewCursor(limit int, after string) *Cursor {
	if limit <= 0 && after == "" {
		return nil
	}
	c := &Cursor{Next: after}
	if limit > 0 {
		c.Limit = limit
	}
	return c
}
