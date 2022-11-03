package handlers

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var h Handlers

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	/// Minio
	minioC, err := minio.SetupMinio(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	configsBucketKey := minio.GetConfigsBucketKey()
	layersBucketKey := minio.GetLayersBucketKey()
	manifestsBucketKey := minio.GetManifestsBucketKey()
	bc := map[string]minio.BucketConfig{
		configsBucketKey:   minio.NewBucketConfig(configsBucketKey, minioC.HttpURI),
		layersBucketKey:    minio.NewBucketConfig(layersBucketKey, minioC.HttpURI),
		manifestsBucketKey: minio.NewBucketConfig(manifestsBucketKey, minioC.HttpURI),
	}
	min, err := minio.NewMiniWithCfg(bc)
	if err != nil {
		logrus.Fatal(err)
	}
	err = min.ConnectClients()
	if err != nil {
		logrus.Fatal(err)
	}
	min.BucketCheck()
	/// SQLite
	f, err := os.CreateTemp("", "sqlite_*.db")
	if err != nil {
		logrus.Fatal(err)
	}
	defer os.Remove(f.Name())
	_, err = copy("../../../../misc/sqlite/metahub_base.db", f.Name())
	if err != nil {
		logrus.Fatal(err)
	}
	sq, err := sqlite.New(f.Name(), "")
	if err != nil {
		logrus.Fatal(err)
	}
	mag := magazine.Magazine{
		Minio:  min,
		SQlite: sq,
	}
	h = Handlers{
		mag: mag,
	}
	code := m.Run()
	defer minioC.Terminate(ctx)
	logrus.Info("End TestMain")
	os.Exit(code)
}

func TeardownTest(bs utils.Bootstrap) {
	logrus.Info("> TeardownTest")
	if bs.CopySqlite != "" {
		logrus.Infof("Copy SQlite DB to '%s'", bs.CopySqlite)
		h.mag.SQlite.Close()
		utils.CopyFile(h.mag.SQlite.MhFile, bs.CopySqlite)
		err := h.mag.SQlite.ReOpenDB()
		if err != nil {
			logrus.Fatal(err)
		}
	}
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		logrus.Fatal(err)
	}
	err = h.mag.SQlite.EmptyData(bs.SqlEmptyAll)
	if err != nil {
		logrus.Fatal(err)
	}
	for _, b := range bs.Bootstraps {
		switch b {
		case "layer-redirect:qnib/test":
			lBc := h.mag.Minio.BucketConfig["layers"]
			lBc.Redirect = false
			h.mag.Minio.BucketConfig["layers"] = lBc
		}
	}
}

func SetupTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	logrus.Info("> SetupTest")
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		logrus.Errorf("SetupTest: %s", b)
		sq := h.mag.SQlite
		switch b {
		case "profiles":
			err = sqlite.Setup_Profiles(&sq, &br, "../../../../misc/yaml/profiles")
			if err != nil {
				return
			}
		case "platforms":
			err = sqlite.Setup_Platforms(&sq, &br)
			if err != nil {
				return
			}
		case "users":
			err = sqlite.Setup_Users(&sq, &br)
			if err != nil {
				return
			}
		case "manifest:qnib/featuretest-arm64":
			err = sqlite.Setup_Manifest_QNIB_FeatureTest_arm(&sq, &br)
			if err != nil {
				return
			}
		case "config:qnib/featuretest-arm64":
			err = sqlite.Setup_Config_FTarm(&sq, &br)
			if err != nil {
				return
			}
		case "upstreams:qnib/featuretest-arm64":
			err = sqlite.Setup_Upstream_FTarm(&sq, &br)
			if err != nil {
				return
			}
		case "layers:qnib/featuretest-arm64":
			err = sqlite.Setup_Layers_FTarm(&sq, &br)
			if err != nil {
				return
			}
		case "manifestlist:qnib/featuretest-generic-arm64":
			err = sqlite.Setup_Manifestlist_FTarm(&sq, &br)
			if err != nil {
				return
			}
		case "collection:qnib/featuretest":
			err = sqlite.Setup_Collection_QNIB_FeatureTest(&sq, &br)
			if err != nil {
				return
			}
		case "filter:generic":
			err = sqlite.Setup_Filter_Generic(&sq, &br)
			if err != nil {
				return
			}
		case "layer:qnib/test":
			// Bootstrap
			L1 := l1
			repo := "qnib"
			image := "test"
			bu1 := types.OCIBlobUpload{
				Digest:     L1.Digest,
				UUID:       uuid.New(),
				UploadedBy: "test",
				Repo:       fmt.Sprintf("%s/%s", repo, image),
				BlobType:   "layer",
			}
			err = h.mag.StoreLayerUpload(ctx, &bu1, L1.Data)
			if err != nil {
				return br, err
			}
		case "layer-redirect:qnib/test":
			// Bootstrap
			L1 := l1
			repo := "qnib"
			image := "test"
			bu1 := types.OCIBlobUpload{
				Digest:     L1.Digest,
				UUID:       uuid.New(),
				UploadedBy: "test",
				Repo:       fmt.Sprintf("%s/%s", repo, image),
				BlobType:   "layer",
			}
			lBc := h.mag.Minio.BucketConfig["layers"]
			lBc.Redirect = true
			h.mag.Minio.BucketConfig["layers"] = lBc
			err = h.mag.StoreLayerUpload(ctx, &bu1, L1.Data)
			if err != nil {
				return br, err
			}
		}
	}
	return
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}
