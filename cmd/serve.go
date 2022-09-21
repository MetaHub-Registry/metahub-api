package cmd

import (
	"fmt"

	"log"
	"net/http"

	"github.com/urfave/cli/v2"
	"github.com/metahub-registry/metahub-api/api/v1/router"
	"github.com/metahub-registry/metahub-api/pkg/config"
)

func ServeApi(cCtx *cli.Context) error {
	/*dbHost := cCtx.String("db-host")
	dbUser := cCtx.String("db-user")
	dbPort := cCtx.String("db-port")
	dbPass := cCtx.String("db-pass")
	dbName := cCtx.String("db-name")
	*/
	port := cCtx.Int("port")
	// NEO4J
	ncfg := config.NeoConfig{
		DbName: cCtx.String("neo-name"),
		User:   cCtx.String("neo-user"),
		Passwd: cCtx.String("neo-pass"),
		URL:    cCtx.String("neo-url"),
	}
	// SQL DB
	/*
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable timezone=UTC connect_timeout=5", dbHost, dbPort, dbUser, dbPass, dbName)
		db, err := driver.ConnectPostgres(dsn)
		if err != nil {
			log.Fatal("Cannot connect to database")
		}
		defer db.SQL.Close()
	*/
	r := router.Initialize(ncfg)
	addr := fmt.Sprintf(":%d", port)
	err := http.ListenAndServe(addr, r)
	if err != nil {
		log.Fatal("Error serving router: ", err)
	}
	return nil
}
