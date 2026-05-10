package types

import "time"

// OciRepo is a Massdriver OCI repository — a named container in the
// organization's catalog that holds versioned OCI artifacts (today: bundles).
//
// The repository can be addressed two ways:
//   - As a GraphQL record: query/mutate via massdriver/platform/ocirepos
//     CRUD functions (Get, List, Create, Update, Delete).
//   - As an OCI registry endpoint: get an oras.Target via
//     ocirepos.Target(client, name) for pulling/pushing artifacts directly.
//
// `Description`, `Icon`, `SourceURL`, `Readme`, and `Changelog` are surfaced
// from the latest published release; they are nil/empty until the repository
// has at least one tag.
type OciRepo struct {
	ID           string         `json:"id" mapstructure:"id"`
	Name         string         `json:"name" mapstructure:"name"`
	Reference    string         `json:"reference,omitempty" mapstructure:"reference"`
	ArtifactType string         `json:"artifactType" mapstructure:"artifactType"`
	Attributes   map[string]any `json:"attributes,omitempty" mapstructure:"attributes,omitempty"`
	Description  string         `json:"description,omitempty" mapstructure:"description"`
	Icon         string         `json:"icon,omitempty" mapstructure:"icon"`
	SourceURL    string         `json:"sourceUrl,omitempty" mapstructure:"sourceUrl"`
	CreatedAt    time.Time      `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}
