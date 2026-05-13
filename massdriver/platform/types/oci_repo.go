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
	CreatedAt       time.Time               `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt       time.Time               `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	Tags            []OciRepoTag            `json:"tags,omitempty" mapstructure:"-"`
	ReleaseChannels []OciRepoReleaseChannel `json:"releaseChannels,omitempty" mapstructure:"-"`
	LatestTag       string                  `json:"latestTag,omitempty" mapstructure:"-"`
}

// OciRepoTag is one published version in an [OciRepo].
type OciRepoTag struct {
	Tag       string    `json:"tag" mapstructure:"tag"`
	CreatedAt time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
}

// OciRepoReleaseChannel is an auto-resolving version constraint on an
// [OciRepo]. Name is the constraint expression (e.g. "~1", "latest");
// Tag is the resolved semver the channel currently points to.
type OciRepoReleaseChannel struct {
	Name string `json:"name" mapstructure:"name"`
	Tag  string `json:"tag" mapstructure:"tag"`
}
