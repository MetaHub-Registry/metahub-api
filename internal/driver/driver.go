package driver

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

type DB struct {
	SQL *sql.DB
}

var dbConn = &DB{}

const maxOpenDbConn = 10
const maxIdleDbConn = 10
const maxDbLifeTime = 5 * time.Minute

func ConnectPostgres(dsn string) (*DB, error) {
	d, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	d.SetMaxOpenConns(maxOpenDbConn)
	d.SetMaxIdleConns(maxIdleDbConn)
	d.SetConnMaxLifetime(maxDbLifeTime)

	err = testDB(d)
	if err != nil {
		return nil, err
	}

	dbConn.SQL = d
	return dbConn, nil
}

func ConnectSqlite(file string) (*DB, error) {
	logrus.Info("Connect to sqlite database: ", file)

	d, err := sql.Open("sqlite3", file)
	if err != nil {
		return nil, err
	}

	/*
		d.SetMaxOpenConns(maxOpenDbConn)
		d.SetMaxIdleConns(maxIdleDbConn)
		d.SetConnMaxLifetime(maxDbLifeTime)
	*/
	err = testDB(d)
	if err != nil {
		return nil, err
	}

	dbConn.SQL = d
	return dbConn, nil
}

func testDB(d *sql.DB) error {
	err := d.Ping()
	if err != nil {
		fmt.Println("Error!", err)
		return err
	}
	fmt.Println("*** Pinged database successfully! ***")

	return nil
}
