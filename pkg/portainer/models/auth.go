package models

// AuthResponse represents the response from an authentication request.
type AuthResponse struct {
	// JWT token used to authenticate against the API
	JWT string `json:"jwt"`
}
