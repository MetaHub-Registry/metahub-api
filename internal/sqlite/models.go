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
	//User       User
	//Token      Token
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
		Profile:    Profile{},
		Filter:     Filter{},
		Platform:   Platform{},
		Upstream:   Upstream{},
		Config:     Config{},
		Manifest:   Manifest{},
		Collection: Collection{},
	}
	m.createTables()
	return
}

func (m Models) createTables() {
	logrus.Info(">> Create tables")
	var err error
	err = m.Profile.createTable()
	if err != nil {
		logrus.Errorf("while Profile.createTable: %v", err)
	}
	err = m.Filter.createTable()
	if err != nil {
		logrus.Errorf("while Filter.createTable: %v", err)
	}
	err = m.Platform.createTable()
	if err != nil {
		logrus.Errorf("while Platform.createTable: %v", err)
	}
	err = m.Config.createTable()
	if err != nil {
		logrus.Errorf("while Config.createTable: %v", err)
	}
	err = m.Upstream.createTable()
	if err != nil {
		logrus.Errorf("while Upstream.createTable: %v", err)
	}
	err = m.Manifest.createTable()
	if err != nil {
		logrus.Errorf("while Manifest.createTable: %v", err)
	}
	err = m.Collection.createTable()
	if err != nil {
		logrus.Errorf("while Collection.createTable: %v", err)
	}
	logrus.Info(">> Create tables done")

}
