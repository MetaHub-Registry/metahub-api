package types

type User struct {
	SqlID        int64  `json:"sql_id"`
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"hash"`
	Email        string `yaml:"email"`
	Active       int    `yaml:"active"`
	FirstName    string `yaml:"first_name,omitempty"`
	LastName     string `yaml:"last_name,omitempty"`
}
