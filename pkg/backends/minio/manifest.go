package minio

import (
	"context"
	"io"

	"github.com/containers/image/manifest"
	"github.com/minio/minio-go/v7"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func (m Minio) ManifestGetObj(ctx context.Context, path string) (mani types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "backend/minio/manifest.go",
		"func":       "ManifestGetObj",
		"path":       path,
	})
	cli := m.BucketConfig[manifestsBucketKey].Client
	obj, err := cli.GetObject(ctx, m.GetManifestsBucketName(), path, minio.GetObjectOptions{})
	if err != nil {
		logrus.Errorf("Error getting manifest: %s", err)
		return
	}
	defer obj.Close()
	b, e := io.ReadAll(obj)
	if e != nil {
		logrus.Error(e)
	}
	mani, err = m.NewManifestFromBytes(b)
	if err != nil {
		err = errors.Wrapf(err, "m.NewManifestFromBytes")
		l.Error(err)
	}
	return
}

func (m Minio) NewManifestFromBytes(b []byte) (mani types.Manifest, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "backend/minio/manifest.go",
		"func":       "NewManifestFromBytes",
	})
	mt := manifest.GuessMIMEType(b)
	cntMani, err := manifest.FromBlob(b, mt)
	if err != nil {
		err = errors.WithMessagef(err, "manifest.FromBlob %s", string(b))
		l.Error(err)
	}
	bInfo := cntMani.ConfigInfo()
	// TODO: Is deriving digest from byte safe?
	maniDig := digest.FromBytes(b)
	mani = types.Manifest{
		// We need to store the byte array as the digest will change if we marhsal our own
		Data:      b,
		MediaType: mt,
		Digest:    maniDig.String(),
		Size:      bInfo.Size,
	}
	imgInfo, err := cntMani.Inspect(m.configGetter)
	if err != nil {
		err = errors.Wrapf(err, "failed to inspect manifest")
		l.Error(err)
	}
	mani.Config = types.Config{
		Digest:    bInfo.Digest.String(),
		Size:      bInfo.Size,
		MediaType: bInfo.MediaType,
	}
	mani.Platform = types.Platform{
		Arch: imgInfo.Architecture,
		OS:   imgInfo.Os,
		// TODO: Add Variant (containers/image/v5/types?)
	}
	for _, layer := range cntMani.LayerInfos() {
		mani.Layers = append(mani.Layers, types.Layer{
			MediaType: layer.MediaType,
			Digest:    layer.Digest.String(),
			Size:      layer.Size,
		})
	}
	return
}
