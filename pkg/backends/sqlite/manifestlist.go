package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

// ManifestList serve a collection for a certain profile.
type ManifestList struct {
	ID     int64  `json:"id"`
	Digest string `json:"digest"`
	// key of the map matches the key of the map in Platforms to construct the platformObject indexes of the ManifestList
	Manifests map[string]Manifest `json:"manifests"`
	Platforms map[string]Platform `json:"platform"`
}

func (ml ManifestList) CreateTables() (err error) {
	err = ml.createManifestlistTable()
	if err != nil {
		return
	}
	err = ml.createManifestListXTable()
	if err != nil {
		return
	}
	return
}

func (ml ManifestList) createManifestListXTable() (err error) {
	tableName := "manifestlist_xtable"
	q := fmt.Sprintf(`PRAGMA table_info(%s)`, tableName)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		logrus.Infof("Table '%s' already exists!", tableName)
		return
	}
	s := `CREATE TABLE IF NOT EXISTS %s (
		    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "manifest_id" INT, 
		"manifestlist_id" INT,
		FOREIGN KEY (manifest_id)
			REFERENCES manifest (id)
		FOREIGN KEY (manifestlist_id)
			REFERENCES manifestlist (id)
		);`
	stmt := fmt.Sprintf(s, tableName)
	query, err := db.Prepare(stmt)
	if err != nil {
		logrus.Fatal(err)
	}
	_, err = query.Exec()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Table '%s' created successfully!", tableName)
	return
}

func (ml ManifestList) createManifestlistTable() (err error) {
	tableName := "manifestlist"
	q := fmt.Sprintf(`PRAGMA table_info(%s)`, tableName)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		logrus.Infof("Table '%s' already exists!", tableName)
		return
	}
	s := `CREATE TABLE IF NOT EXISTS %s (
		    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "digest" TEXT,
		"media_type" TEXT,
		"size" INT
		);`
	stmt := fmt.Sprintf(s, tableName)
	query, err := db.Prepare(stmt)
	if err != nil {
		logrus.Fatal(err)
	}
	_, err = query.Exec()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Table '%s' created successfully!", tableName)
	return
}

func (ml ManifestList) Insert(i *types.ManifestList) (err error) {
	q := `INSERT INTO manifestlist (digest, media_type) VALUES (?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(i.Digest, i.MediaType)
	if err != nil {
		logrus.Error(err)
		return
	}
	i.SqlID, err = res.LastInsertId()
	for _, m := range i.Manifests {
		// Get SqlID of manifest
		q := `SELECT id FROM manifest WHERE digest=?`
		stmt, err = db.Prepare(q)
		if err != nil {
			logrus.Error(err)
			return err
		}
		var maniSqlID int64
		err = stmt.QueryRow(m.Digest).Scan(&maniSqlID)
		if err != nil {
			logrus.Error(err)
			return err
		}

		q = `INSERT INTO manifestlist_xtable (manifest_id, manifestlist_id) VALUES (?, ?)`
		stmt, err = db.Prepare(q)
		if err != nil {
			logrus.Error(err)
			return
		}
		_, err = stmt.Exec(maniSqlID, i.SqlID)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	return
}

func (ml ManifestList) GetTypesManifestListByRequest(req types.Request) (res types.ManifestList, found bool, err error) {
	q := fmt.Sprintf(`SELECT DISTINCT colID,colRepo,colTag,manifestlistDigest,manifestlistID FROM v_get_manifestlist WHERE colRepo="%s" AND colTag="%s" AND profileName="%s"`, req.Repo, req.Tag, req.Profile)
	if req.Tag == "" {
		q = fmt.Sprintf(`SELECT DISTINCT colID,colRepo,colTag,manifestlistDigest,manifestlistID FROM v_get_manifestlist WHERE colRepo="%s" AND manifestlistDigest="%s" AND profileName="%s"`, req.Repo, req.Digest, req.Profile)
	}
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	// BootStrap the ML
	res = types.ManifestList{
		Digest:        "",
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
		Manifests:     []types.Manifest{},
	}
	for rows.Next() {
		found = true
		r := mlRes{}
		err = rows.Scan(
			&r.ColID,
			&r.ColRepo,
			&r.ColTag,
			&r.MlDigest,
			&r.MlID,
		)
		if err != nil {
			logrus.Error(err)
			continue
		}
		res.SqlID = int64(r.MlID)
		if res.Digest == "" {
			// First row
			res.Digest = r.MlDigest
		}
	}
	return
}
