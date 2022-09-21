package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/metahub-registry/metahub-api/cmd"
)

func main() {
	app := &cli.App{
		Name: "metahub-api",
		Commands: []*cli.Command{
			{
				Name:   "serve",
				Usage:  "Serves the API",
				Action: cmd.ServeApi,
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "db-host", EnvVars: []string{"DB_HOST"}, Value: "localhost", Usage: "DB host"},
					&cli.StringFlag{Name: "db-user", EnvVars: []string{"DB_USER"}, Value: "postgres", Usage: "DB user"},
					&cli.StringFlag{Name: "db-port", EnvVars: []string{"DB_PORT"}, Value: "5432", Usage: "DB port"},
					&cli.StringFlag{Name: "db-pass", EnvVars: []string{"DB_PASS"}, Value: "", Usage: "DB password"},
					&cli.StringFlag{Name: "db-name", EnvVars: []string{"DB_NAME"}, Value: "metahub", Usage: "DB database name"},
					&cli.IntFlag{Name: "port", EnvVars: []string{"API_PORT"}, Value: 8080, Usage: "Port to listen on"},
				},
			}, {
				Name:   "version",
				Usage:  "Prints the version",
				Action: cmd.Version,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
