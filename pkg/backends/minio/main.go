package minio

import (
	"strings"

	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/credentials"
	"github.com/sirupsen/logrus"
)

var localPath = "/tmp/metahub-api-sqlite.db"

func FetchObject(path string) (err error) {
	logrus.Infof("Fetch object '%s'", path)
	iam := credentials.NewIAM("")
	cli, err := minio.NewWithCredentials("s3.amazonaws.com", iam, false, "")
	if err != nil {
		return
	}
	p := strings.TrimPrefix(path, "s3://")
	slc := strings.Split(p, "/")
	bucketName := slc[0]
	bucketPath := strings.Join(slc[1:], "/")
	err = cli.FGetObject(bucketName, bucketPath, localPath, minio.GetObjectOptions{})
	if err != nil {
		return
	}
	return
}
