package magazine

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func (mag Magazine) Manifest_GetByHTTPRequest(r *http.Request) (m types.Manifest, found bool, err error) {
	req := types.NewRequest(r)
	msg := fmt.Sprintf("%-5s ManifestHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	if req.Profile != "generic" {
		msg += fmt.Sprintf(" - profile: %s", req.Profile)
	}
	logrus.Info(msg)
	m, found, err = mag.Manifest_GetByRequest(req)
	if err != nil {
		return
	}
	return
}

func (mag Magazine) Manifest_GetByRequest(req types.Request) (m types.Manifest, found bool, err error) {
	ctx := context.Background()
	// Fetch Manifest from SQLite
	m, found, err = mag.SQlite.Manifest.GetTypesManifestByRequest(req)
	if err != nil {
		err = errors.Wrap(err, "failed to get Manifest from SQLite")
		return
	}
	if !found {
		return
	}
	m, err = mag.Minio.ManifestGetObj(ctx, m.Digest)
	return
}
