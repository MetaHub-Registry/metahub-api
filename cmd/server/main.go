package main

import (
	"log"
	"net/http"
	"os"

	"github.com/metahub-registry/metahub-api/api/v1/router"
	"github.com/metahub-registry/metahub-api/internal/driver"
)

func main() {
	dsn := os.Getenv("DSN")
	db, err := driver.ConnectPostgres(dsn)
	if err != nil {
		log.Fatal("Cannot connect to database")
	}
	defer db.SQL.Close()

	r := router.Initialize(db)
	err = http.ListenAndServe(":8080", r)
	if err != nil {
		log.Fatal("Error serving router: ", err)
	}
}
