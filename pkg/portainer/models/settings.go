package models

import apimodels "github.com/portainer/client-api-go/v2/pkg/models"

// PortainerSettings represents the global Portainer instance settings.
type PortainerSettings struct {
	Authentication struct {
		Method string `json:"method"`
	} `json:"authentication"`
	Edge struct {
		Enabled   bool   `json:"enabled"`
		ServerURL string `json:"server_url"`
	} `json:"edge"`
	LogoURL                string `json:"logo_url,omitempty"`
	EnableTelemetry        bool   `json:"enable_telemetry"`
	RequiredPasswordLength int    `json:"required_password_length"`
}

const (
	AuthenticationMethodInternal = "internal"
	AuthenticationMethodLDAP     = "ldap"
	AuthenticationMethodOAuth    = "oauth"
	AuthenticationMethodUnknown  = "unknown"
)

// ConvertSettingsToPortainerSettings converts raw Portainer settings into a simplified PortainerSettings model.
func ConvertSettingsToPortainerSettings(rawSettings *apimodels.PortainereeSettings) PortainerSettings {
	if rawSettings == nil {
		return PortainerSettings{}
	}

	s := PortainerSettings{}

	s.Authentication.Method = convertAuthenticationMethod(rawSettings.AuthenticationMethod)
	s.Edge.Enabled = rawSettings.EnableEdgeComputeFeatures
	if rawSettings.Edge != nil {
		s.Edge.ServerURL = rawSettings.Edge.TunnelServerAddress
	}
	s.LogoURL = rawSettings.LogoURL
	s.EnableTelemetry = rawSettings.EnableTelemetry
	if rawSettings.InternalAuthSettings != nil {
		s.RequiredPasswordLength = int(rawSettings.InternalAuthSettings.RequiredPasswordLength)
	}

	return s
}

// PublicSettings represents the publicly available settings of the Portainer instance.
type PublicSettings struct {
	AuthenticationMethod      string          `json:"authentication_method"`
	EnableEdgeComputeFeatures bool            `json:"enable_edge_compute_features"`
	EnableTelemetry           bool            `json:"enable_telemetry"`
	LogoURL                   string          `json:"logo_url"`
	OAuthLoginURI             string          `json:"oauth_login_uri,omitempty"`
	OAuthLogoutURI            string          `json:"oauth_logout_uri,omitempty"`
	OAuthHideInternalAuth     bool            `json:"oauth_hide_internal_auth"`
	RequiredPasswordLength    int             `json:"required_password_length"`
	Features                  map[string]bool `json:"features,omitempty"`
}

// ConvertToPublicSettings converts a raw SDK public settings response to the local PublicSettings model.
func ConvertToPublicSettings(raw *apimodels.SettingsPublicSettingsResponse) PublicSettings {
	if raw == nil {
		return PublicSettings{}
	}

	return PublicSettings{
		AuthenticationMethod:      convertAuthenticationMethod(raw.AuthenticationMethod),
		EnableEdgeComputeFeatures: raw.EnableEdgeComputeFeatures,
		EnableTelemetry:           raw.EnableTelemetry,
		LogoURL:                   raw.LogoURL,
		OAuthLoginURI:             raw.OAuthLoginURI,
		OAuthLogoutURI:            raw.OAuthLogoutURI,
		OAuthHideInternalAuth:     raw.OAuthHideInternalAuth,
		RequiredPasswordLength:    int(raw.RequiredPasswordLength),
		Features:                  raw.Features,
	}
}

func convertAuthenticationMethod(method int64) string {
	switch method {
	case 1:
		return AuthenticationMethodInternal
	case 2:
		return AuthenticationMethodLDAP
	case 3:
		return AuthenticationMethodOAuth
	default:
		return AuthenticationMethodUnknown
	}
}
