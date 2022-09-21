package data

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/config"
)

const dbTimeout = time.Second * 3

var db *sql.DB
var neoDriver neo4j.Driver

// Models is the type for this package. Note that any model that is included as a member
// in this type is available to us throughout the application, anywhere that the
// app variable is used, provided that the model is also added in the New function.
type Models struct {
	User       User
	Token      Token
	Collection Collection
	Manifest   Manifest
	NeoConfig  config.NeoConfig
}

// New is the function used to create an instance of the data package. It returns the type
// Model, which embeds all of the types we want to be available to our application.
func New(dbPool *sql.DB, ncfg config.NeoConfig) (m Models, err error) {
	db = dbPool
	if ncfg.URL == "" {
		err = fmt.Errorf("neo4j url not set")
		return
	}
	nd, err := neo4j.NewDriver(ncfg.URL, neo4j.BasicAuth(ncfg.User, ncfg.Passwd, ""))
	if err != nil {
		logrus.Errorf("Error creating neo driver: %s\n", err)
		return
	}
	neoDriver = nd

	return Models{
		User:  User{},
		Token: Token{},
	}, nil
}
