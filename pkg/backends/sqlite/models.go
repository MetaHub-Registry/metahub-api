package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var db *sql.DB

const dbTimeout = time.Second * 3

// Models is the type for this package. Note that any model that is included as a member
// in this type is available to us throughout the application, anywhere that the
// app variable is used, provided that the model is also added in the New function.
type Models struct {
	Token        Token
	User         User
	Profile      Profile
	Filter       Filter
	Platform     Platform
	Collection   Collection
	Manifest     Manifest
	ManifestList ManifestList
	Config       Config
	Upstream     Upstream
}

// New is the function used to create an instance of the data package. It returns the type
// Model, which embeds all of the types we want to be available to our application.
func New(dbPool *sql.DB) (m Models, err error) {
	db = dbPool
	m = Models{
		User:         User{},
		Token:        Token{},
		Profile:      Profile{},
		Filter:       Filter{},
		Platform:     Platform{},
		Upstream:     Upstream{},
		Config:       Config{},
		Manifest:     Manifest{},
		ManifestList: ManifestList{},
		Collection:   Collection{},
	}
	return
}

func (m Models) LoginUser(username, profile, password string) (err error) {
	logrus.Debug(">> LoginUser")
	// check login and return token
	u, err := m.User.GetByUsername(username)
	if err != nil {
		return fmt.Errorf("User not found: %s", err.Error())
	}
	match, err := u.PasswordMatches(password)
	if err != nil {
		err = errors.Wrap(err, "when matching password")
		logrus.Error(err)
		return fmt.Errorf("error when matching passwords")
	}
	if !match {
		return fmt.Errorf("password does not match")
	}
	return
}
