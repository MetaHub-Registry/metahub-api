package cmd

import (
	"errors"
	"fmt"
	"os"

	"log"
	"net/http"

	"github.com/urfave/cli/v2"
	"github.com/metahub-registry/metahub-api/api/v1/router"
	"github.com/metahub-registry/metahub-api/internal/driver"
)

func ServeApi(cCtx *cli.Context) error {
	/*dbHost := cCtx.String("db-host")
	dbUser := cCtx.String("db-user")
	dbPort := cCtx.String("db-port")
	dbPass := cCtx.String("db-pass")
	dbName := cCtx.String("db-name")
	*/
	port := cCtx.Int("port")
	fname := cCtx.String("sqlite-file")
	// SQL DB
	if fname == "" {
		log.Fatal("No sqlite-file given")
	}
	if _, err := os.Stat(fname); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("sqlite-file '%s'does not exist", fname)
		} else {
			log.Fatalf("Error opening sqlite file: %s", err)
		}
	}
	db, err := driver.ConnectSqlite(fname)
	if err != nil {
		log.Fatal("Cannot connect to database")
	}
	defer db.SQL.Close()
	r := router.Initialize(db)
	addr := fmt.Sprintf(":%d", port)
	err = http.ListenAndServe(addr, r)
	if err != nil {
		log.Fatal("Error serving router: ", err)
	}
	return nil
}
