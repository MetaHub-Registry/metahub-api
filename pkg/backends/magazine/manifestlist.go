package magazine

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func (mag Magazine) ManifestListGetByRequest(req types.Request) (ml types.ManifestList, found bool, err error) {
	logrus.Debugf("ManifestListGetByReqest: %v", req)
	ctx := context.Background()
	// Fetch ML from SQLite
	ml, found, err = mag.SQlite.ManifestList.GetTypesManifestListByRequest(req)
	if err != nil {
		logrus.Errorf("Failed to get ManifestList from SQLite: %v", err)
		return
	}
	if !found {
		// We did not find a ManifestList in SQLite
		return
	}
	manis, err := mag.SQlite.Manifest.TypesManifest_GetByManifestListID(ml.SqlID)
	for idx, m := range manis {
		// Fetch Manifest from Minio
		logrus.Warnf("Mani[%d]: %v", idx, m)
		miniMani, err := mag.Minio.ManifestGetObj(ctx, m.Digest)
		if err != nil {
			logrus.Errorf("Failed to fetch Manifest from Object store: %v", err)
			return ml, found, err
		}
		ml.Manifests = append(ml.Manifests, miniMani)
	}
	return
}

func (mag Magazine) ManifestList_GetByHTTPRequest(r *http.Request) (ml types.ManifestList, found bool, err error) {
	req := types.NewRequest(r)
	msg := fmt.Sprintf("%-5s ManifestListHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	if req.Profile != "generic" {
		msg += fmt.Sprintf(" - profile: %s", req.Profile)
	}
	logrus.Info(msg)
	ml, found, err = mag.ManifestListGetByRequest(req)
	if err != nil {
		return
	}
	return
}
