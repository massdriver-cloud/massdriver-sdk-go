package types

// AttributeFilter narrows a list by a single effective attribute. Effective
// attributes are a resource's own attributes plus any it inherits from its
// ancestors (project → environment → component → instance); a `team` attribute
// set on a project therefore also matches that project's environments and
// instances.
//
// Key is the attribute key to match (e.g. "team"). Set at most one of Eq or In:
// Eq matches an exact value, In matches any value in the list. Passing several
// AttributeFilters to a list call ANDs them together.
type AttributeFilter struct {
	// Key is the attribute key to match, e.g. "team" or "cost_center".
	Key string
	// Eq matches resources whose value for Key exactly equals this string.
	Eq string
	// In matches resources whose value for Key is any string in this list.
	In []string
}
