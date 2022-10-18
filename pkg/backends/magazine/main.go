package magazine

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/metahub-registry/metahub-api/pkg/backends/minio"
	"github.com/metahub-registry/metahub-api/pkg/backends/sqlite"
)

type Magazine struct {
	Minio  minio.Minio
	SQlite sqlite.Models
}

func NewMagazine(cmd *cobra.Command, tables string) (m Magazine, err error) {
	cfgFile, _ := cmd.Flags().GetString("config")
	min, err := minio.NewMinio(cfgFile)
	if err != nil {
		err = errors.Wrapf(err, "failed to create minio backend")
		logrus.Fatal(err)
	}
	err = min.BucketCheck()
	if err != nil {
		err = errors.Wrapf(err, "failed to execute BucketCheck")
		logrus.Fatal(err)
	}
	m = Magazine{
		Minio: min,
	}

	// SQLite
	mhSqlFile, _ := cmd.Flags().GetString("metahub-db")
	db, err := sql.Open("sqlite3", mhSqlFile)
	if err != nil {
		err = errors.Wrapf(err, "failed to open '%s'", mhSqlFile)
		logrus.Fatal(err)
	}
	sqlite, err := sqlite.New(db)
	if err != nil {
		err = errors.Wrapf(err, "failed to create sqlite backend")
		logrus.Fatal(err)
	}
	m.SQlite = sqlite
	return
}
