package data

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"

	"github.com/metahub-registry/metahub-api/internal/utils"
)

type Collection struct {
	ID            int64               `json:"id"`
	Repo          string              `json:"repo"`
	Tag           string              `json:"tag"`
	Manifests     map[string]Manifest `json:"manifests"`
	ManifestCount int64               `json:"manifest_count"`
}

func NewCollectionFromKeys(keys map[string]interface{}) (col Collection, err error) {
	col = Collection{
		Manifests:     make(map[string]Manifest),
		ManifestCount: 0,
	}
	if id, ok := keys["id"]; ok {
		col.ID = id.(int64)
	} else {
		err = fmt.Errorf("no key 'id' in keys: %v", keys)
		return
	}
	if repo, ok := keys["repo"]; ok {
		col.Repo = repo.(string)
	} else {
		err = fmt.Errorf("no key 'repo' in keys: %v", keys)
		return
	}
	if tag, ok := keys["tag"]; ok {
		col.Tag = tag.(string)
	} else {
		err = fmt.Errorf("no key 'tag' in keys: %v", keys)
		return
	}
	if manifestCount, ok := keys["manifest_count"]; ok {
		col.ManifestCount = manifestCount.(int64)
	} else {
		err = fmt.Errorf("no key 'manifest_count' in keys: %v", keys)
	}
	return
}

func (c *Collection) GetAll() (cols []Collection, err error) {
	//
	session := neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	query := `MATCH (n:Collection) RETURN id(n) AS id, n.repo AS repo, n.tag AS tag`
	input := map[string]interface{}{}
	result, err := session.Run(query, input)
	if err != nil {
		return
	}
	for result.Next() {
		record := result.Record()
		k, _ := utils.NeoGetKeys(record, []string{"id", "repo", "tag"})
		col, err := NewCollectionFromKeys(k)
		if err != nil {
			logrus.Fatal(err)
		}
		cols = append(cols, col)
	}
	return
}

func (c *Collection) GetCollectionByRepoTag(repo, tag string) (col Collection, err error) {
	logrus.Info("GetCollectionByRepoTag")
	session := neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (n:Collection)-[r:UPSTREAM]->() WHERE n.repo = "%s" AND n.tag = "%s" RETURN id(n) AS id, n.repo AS repo, n.tag AS tag, count(r) AS manifest_count`
	query := fmt.Sprintf(q, repo, tag)
	input := map[string]interface{}{}
	result, err := session.Run(query, input)
	if err != nil {
		return
	}
	if result.Next() {
		record := result.Record()
		k, _ := utils.NeoGetKeys(record, []string{"id", "repo", "tag", "manifest_count"})
		col, err = NewCollectionFromKeys(k)
		if err != nil {
			logrus.Fatal(err)
		}
	}
	return
}

func (c *Collection) GetCollectionByRepo(repo string) (cols []Collection, err error) {
	logrus.Info("GetCollectionByRepo")
	session := neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	query := `MATCH (n:Collection)-[r:UPSTREAM]->() RETURN id(n) AS id, n.repo AS repo, n.tag AS tag, count(r) AS manifest_count`
	if len(repo) > 0 {
		q := `MATCH (n:Collection)-[r:UPSTREAM]->() WHERE n.repo STARTS WITH "%s" RETURN id(n) AS id, n.repo AS repo, n.tag AS tag, count(r) AS manifest_count`
		query = fmt.Sprintf(q, repo)
	}
	input := map[string]interface{}{}
	result, err := session.Run(query, input)
	if err != nil {
		return
	}
	for result.Next() {
		record := result.Record()
		k, _ := utils.NeoGetKeys(record, []string{"id", "repo", "tag", "manifest_count"})
		col, err := NewCollectionFromKeys(k)
		if err != nil {
			logrus.Fatal(err)
		}
		cols = append(cols, col)
	}
	return
}
