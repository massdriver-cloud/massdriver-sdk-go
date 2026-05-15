package types

import "time"

// Group is a collection of users and service accounts that share the same
// access level within an organization. Groups are the primary mechanism
// for managing access control in Massdriver.
//
// Two built-in groups exist on every organization (Admins with role
// ORGANIZATION_ADMIN, Viewers with role ORGANIZATION_VIEWER). Custom
// groups have role CUSTOM and grant project-level access via attached
// policies.
type Group struct {
	ID          string    `json:"id" mapstructure:"id"`
	Name        string    `json:"name" mapstructure:"name"`
	Description string    `json:"description,omitempty" mapstructure:"description"`
	Role        string    `json:"role" mapstructure:"role"`
	CreatedAt   time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}

// GroupInvitation is a pending email invitation to join a [Group]. The
// invited user must accept before they become a member.
type GroupInvitation struct {
	ID        string    `json:"id" mapstructure:"id"`
	Email     string    `json:"email" mapstructure:"email"`
	CreatedAt time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
}
