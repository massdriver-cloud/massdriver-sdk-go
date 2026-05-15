package types

// Connection is the runtime wiring between two deployed instances within an
// [Environment] — the realization of a [Link] from the project blueprint.
//
// Skeleton — fuller shape will land alongside the platform/instances package.
type Connection struct {
	ID        string `json:"id" mapstructure:"id"`
	FromField string `json:"fromField" mapstructure:"fromField"`
	ToField   string `json:"toField" mapstructure:"toField"`
}
