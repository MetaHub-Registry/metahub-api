package account

import (
	"encoding/base64"
)

// AccessToken are used to make API requests on behalf of a user.
// The access token represents the authorization of a specific application to access
// specific parts of a user’s data. It's usually the base64 encoded user:pass
type AccessToken struct {
	Token string `json:"token"`
}

// GetUser returns the username and password (to check) based on the token
func (at *AccessToken) GetUser() string {
	decoded, _ := base64.StdEncoding.DecodeString(at.Token)
	return string(decoded)
}
