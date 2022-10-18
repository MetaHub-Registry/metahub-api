package minio

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func (m Minio) LayerGetBytes(ctx context.Context, path string) (b []byte, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "LayerGetBytes",
	})
	_ = l
	cli := m.BucketConfig[layerBucket].Client
	obj, err := cli.GetObject(ctx, layerBucket, path, minio.GetObjectOptions{})
	if err != nil {
		logrus.Errorf("Error getting layer: %s", err)
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		err = errors.Wrap(e, "failed to read layer")
		logrus.Fatal(err)
	}
	return
}

func (m Minio) Layers_GetByManifestDigest(dig string) (layers []types.Layer, err error) {

	return
}
