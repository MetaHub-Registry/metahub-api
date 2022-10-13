package sqlite

import (
	"database/sql"
	"time"

	"github.com/sirupsen/logrus"
)

var db *sql.DB

const dbTimeout = time.Second * 3

// Models is the type for this package. Note that any model that is included as a member
// in this type is available to us throughout the application, anywhere that the
// app variable is used, provided that the model is also added in the New function.
type Models struct {
	Token      Token
	User       User
	Profile    Profile
	Filter     Filter
	Platform   Platform
	Collection Collection
	Manifest   Manifest
	Config     Config
	Upstream   Upstream
}

// New is the function used to create an instance of the data package. It returns the type
// Model, which embeds all of the types we want to be available to our application.
func New(dbPool *sql.DB) (m Models, err error) {
	db = dbPool
	m = Models{
		User:       User{},
		Token:      Token{},
		Profile:    Profile{},
		Filter:     Filter{},
		Platform:   Platform{},
		Upstream:   Upstream{},
		Config:     Config{},
		Manifest:   Manifest{},
		Collection: Collection{},
	}
	m.createViews()
	return
}

func (m Models) createViews() (err error) {
	logrus.Debug(">> Create views")
	if err = m.Collection.CreateViews(); err != nil {
		return
	}
	if err = m.Manifest.CreateViews(); err != nil {
		return
	}
	return
}
