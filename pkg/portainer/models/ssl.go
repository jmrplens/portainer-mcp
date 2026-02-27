package models

import apimodels "github.com/portainer/client-api-go/v2/pkg/models"

// SSLSettings represents the SSL settings of the Portainer instance.
type SSLSettings struct {
	CertPath    string `json:"cert_path"`
	KeyPath     string `json:"key_path"`
	CACertPath  string `json:"ca_cert_path"`
	HTTPEnabled bool   `json:"http_enabled"`
	SelfSigned  bool   `json:"self_signed"`
}

// ConvertToSSLSettings converts raw SDK SSL settings to the local SSLSettings model.
func ConvertToSSLSettings(raw *apimodels.PortainereeSSLSettings) SSLSettings {
	if raw == nil {
		return SSLSettings{}
	}

	return SSLSettings{
		CertPath:    raw.CertPath,
		KeyPath:     raw.KeyPath,
		CACertPath:  raw.CaCertPath,
		HTTPEnabled: raw.HTTPEnabled,
		SelfSigned:  raw.SelfSigned,
	}
}
