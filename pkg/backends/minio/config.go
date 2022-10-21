package minio

import (
	"context"
	"io"

	imgTypes "github.com/containers/image/types"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

func (m Minio) ConfigGetBytes(ctx context.Context, path string) (b []byte, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "ConfigGetObj",
	})
	_ = l
	cli := m.BucketConfig[configsBucketKey].Client
	obj, err := cli.GetObject(ctx, m.GetConfigsBucketName(), path, minio.GetObjectOptions{})
	if err != nil {
		logrus.Errorf("Error getting config: %s", err)
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		return nil, e
	}
	return
}

func (m Minio) configGetter(bi imgTypes.BlobInfo) (b []byte, err error) {
	return m.ConfigGetBytes(context.Background(), bi.Digest.String())
}
