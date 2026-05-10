package types

// Account is a human user record — used in [Group] and [Organization]
// membership lists. Different from [Viewer], which is the
// currently-authenticated entity (account or service account).
type Account struct {
	ID        string `json:"id" mapstructure:"id"`
	Email     string `json:"email" mapstructure:"email"`
	FirstName string `json:"firstName,omitempty" mapstructure:"firstName"`
	LastName  string `json:"lastName,omitempty" mapstructure:"lastName"`
}
