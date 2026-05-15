package types

// Server is metadata about the Massdriver server you're connected to —
// version, mode, and the authentication methods available on the login
// screen. The server query does not require authentication, so this can
// be fetched before a user signs in.
type Server struct {
	// AppURL is the base URL of the Massdriver web application
	// (e.g. "https://app.massdriver.cloud"). Use this when building links
	// to surface in CLIs/tools.
	AppURL string `json:"appUrl" mapstructure:"appUrl"`

	// Version is the server's semantic version (e.g. "1.2.3").
	Version string `json:"version" mapstructure:"version"`

	// Mode is whether this is "self_hosted" or "cloud".
	Mode string `json:"mode" mapstructure:"mode"`

	// SsoProviders are the SSO identity providers configured on this server.
	// Empty when SSO is not configured.
	SsoProviders []SsoProvider `json:"ssoProviders,omitempty" mapstructure:"ssoProviders,omitempty"`

	// EmailAuthMethods are the email-based authentication methods
	// available (e.g. PASSKEY). Empty when only SSO is available.
	EmailAuthMethods []EmailAuthMethod `json:"emailAuthMethods,omitempty" mapstructure:"emailAuthMethods,omitempty"`
}

// SsoProvider is one SSO identity provider configured on the server.
type SsoProvider struct {
	Name      string `json:"name" mapstructure:"name"`
	LoginURL  string `json:"loginUrl" mapstructure:"loginUrl"`
	UIIconURL string `json:"uiIconUrl,omitempty" mapstructure:"uiIconUrl"`
	UILabel   string `json:"uiLabel,omitempty" mapstructure:"uiLabel"`
}

// EmailAuthMethod is one email-based authentication method available on
// the server.
type EmailAuthMethod struct {
	Name string `json:"name" mapstructure:"name"`
}
