package minio

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (m Minio) LayerGetBytes(ctx context.Context, path string) (b []byte, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "LayerGetBytes",
	})
	cli := m.BucketConfig[layersBucketKey].Client
	obj, err := cli.GetObject(ctx, m.GetLayersBucketName(), path, minio.GetObjectOptions{})
	if err != nil {
		l.Errorf("Error getting layer: %s", err)
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		err = errors.Wrap(e, "failed to read layer")
		l.Fatal(err)
	}
	return
}

func (m Minio) LayerGetPreSignedUrl(ctx context.Context, dig string) (psUrl string, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": "backends/minio",
		"func":    "LayerGetPreSignedUrl",
	})
	cli := m.BucketConfig[layersBucketKey].Client
	reqParams := make(url.Values)
	psU, err := cli.PresignedGetObject(
		context.Background(),
		m.GetLayersBucketName(),
		dig,
		time.Second*30*60,
		reqParams)
	if err != nil {
		l.Errorf("Error getting presigned url: %s", err)
		return
	}
	psUrl = psU.String()
	return
}
