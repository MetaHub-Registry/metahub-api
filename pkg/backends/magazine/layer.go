package magazine

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func (mag Magazine) Layer_GetPresignedUrlByHTTPRequest(r *http.Request) (psUrl string, err error) {
	ctx := context.Background()
	req := types.NewRequest(r)
	return mag.Minio.LayerGetPreSignedUrl(ctx, req.Digest)
}

func (mag Magazine) Layer_GetByHTTPRequest(r *http.Request) (layer types.Layer, found bool, err error) {
	ctx := context.Background()

	req := types.NewRequest(r)
	// Fetch Layer from SQLite
	layer, found, err = mag.SQlite.Layer.LayerGetByDigest(req.Digest)
	if err != nil {
		logrus.Errorf("failed to fetch layer from SQLite: %v", err)
		return
	}
	if !found {
		logrus.Errorf("did not find layer with dig: %s", req.Digest)
		return
	}
	// Fetch Blob
	b, err := mag.Minio.LayerGetBytes(ctx, req.Digest)
	switch {
	case err != nil && err.Error() == "The specified key does not exist.":
		// found is false
		err = nil
		return
	case err != nil:
		logrus.Error(err)
		return
	}
	layer.Data = b
	// No error - off we go
	found = true
	return
}
