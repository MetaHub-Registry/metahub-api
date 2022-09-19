package data

import (
	"fmt"
	"strings"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/internal/utils"
)

type Manifest struct {
	ID           int64                     `json:"id"`
	PlatformSpec manifestlist.PlatformSpec `json:"platform_spec"`
	Upstream     Upstream                  `json:"upstream"`
	Config       Config                    `json:"config"`
}

func NewManifestFromKeys(keys map[string]interface{}) (mani Manifest, err error) {
	mani = Manifest{}
	if id, ok := keys["maniId"]; ok {
		mani.ID = id.(int64)
	} else {
		err = fmt.Errorf("no key 'maniId' in keys: %v", keys)
		return
	}
	up := Upstream{}
	if upId, ok := keys["upId"]; ok {
		up.ID = upId.(int64)
	} else {
		err = fmt.Errorf("no key 'upId' in keys: %v", keys)
		return
	}
	if reg, ok := keys["reg"]; ok {
		up.Reg = reg.(string)
	} else {
		err = fmt.Errorf("no key 'reg' in keys: %v", keys)
		return
	}
	if repo, ok := keys["repo"]; ok {
		up.Repo = repo.(string)
	} else {
		err = fmt.Errorf("no key 'repo' in keys: %v", keys)
		return
	}
	if tag, ok := keys["tag"]; ok {
		up.Tag = tag.(string)
	} else {
		err = fmt.Errorf("no key 'tag' in keys: %v", keys)
		return
	}
	mani.Upstream = up
	// Platform
	plat := manifestlist.PlatformSpec{}
	if os, ok := keys["os"]; ok {
		plat.OS = os.(string)
	} else {
		err = fmt.Errorf("no key 'os' in keys: %v", keys)
		return
	}
	if arch, ok := keys["arch"]; ok {
		plat.Architecture = arch.(string)
	} else {
		err = fmt.Errorf("no key 'arch' in keys: %v", keys)
		return
	}
	if variant, ok := keys["variant"]; ok {
		plat.Variant = variant.(string)
	} else {
		err = fmt.Errorf("no key 'variant' in keys: %v", keys)
		return
	}
	if features, ok := keys["features"]; ok {
		plat.Features = strings.Split(features.(string), ",")
	} else {
		err = fmt.Errorf("no key 'features' in keys: %v", keys)
		return
	}
	mani.PlatformSpec = plat

	return
}

func (m *Manifest) GetByCollectionId(colId int64) (manis []Manifest, err error) {
	//
	session := neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	q := `MATCH (c:Collection)-[:UPSTREAM]->(m:Manifest) WHERE id(c) = %d`
	q += ` MATCH (m)-[:UPSTREAM]->(u:Upstream)`
	q += ` MATCH (m)-[:IS]->(p:Platform)`
	q += ` RETURN id(m) AS maniId,id(u) AS upId, u.reg AS reg, u.repo AS repo, u.tag AS tag, `
	q += ` id(p) AS platId, p.os AS os, p.arch AS arch, p.variant AS variant, p.features AS features`

	query := fmt.Sprintf(q, colId)
	input := map[string]interface{}{}
	result, err := session.Run(query, input)
	if err != nil {
		return
	}
	for result.Next() {
		record := result.Record()
		k, _ := utils.NeoGetKeys(record, []string{"maniId", "upId", "reg", "repo", "tag", "platId", "os", "arch", "variant", "features"})
		mani, err := NewManifestFromKeys(k)
		if err != nil {
			logrus.Fatal(err)
		}
		manis = append(manis, mani)
	}
	return
}
