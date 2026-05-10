package types

// ViewerKind discriminates the two flavors of authenticated entity that
// the GraphQL `viewer` query can return.
type ViewerKind string

const (
	// ViewerKindAccount is a human user (PAT or session auth).
	ViewerKindAccount ViewerKind = "account"
	// ViewerKindServiceAccount is a programmatic service account
	// (basic-auth API key or service-account access token).
	ViewerKindServiceAccount ViewerKind = "service_account"
)

// Viewer is the authenticated entity making this request — a flattened
// view over the GraphQL `Viewer` union (AccountViewer | ServiceAccountViewer).
//
// Inspect [Viewer.Kind] to determine which branch you're looking at:
//
//   - [ViewerKindAccount]: Email, FirstName, LastName populated; Name and
//     Description are empty.
//   - [ViewerKindServiceAccount]: Name and Description populated; Email,
//     FirstName, LastName are empty.
//
// Organization is populated on both kinds — for accounts, it's the
// "default" (most recently joined) organization (may be nil if the user
// belongs to none); for service accounts, it's always the owning
// organization.
type Viewer struct {
	Kind         ViewerKind    `json:"kind" mapstructure:"kind"`
	ID           string        `json:"id" mapstructure:"id"`
	Email        string        `json:"email,omitempty" mapstructure:"email,omitempty"`
	FirstName    string        `json:"firstName,omitempty" mapstructure:"firstName,omitempty"`
	LastName     string        `json:"lastName,omitempty" mapstructure:"lastName,omitempty"`
	Name         string        `json:"name,omitempty" mapstructure:"name,omitempty"`
	Description  string        `json:"description,omitempty" mapstructure:"description,omitempty"`
	Organization *Organization `json:"organization,omitempty" mapstructure:"organization,omitempty"`
}
