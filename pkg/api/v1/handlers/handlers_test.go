package handlers

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	neoDb     = "test"
	neoUser   = "neo4j"
	neoPasswd = "test"
)

var h Handlers

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	/// Neo4j
	neoC, err := neo.SetupNeo4j(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer neoC.Terminate(ctx)
	ncfg := neo.NeoConfig{
		DbName: neoDb,
		User:   neoUser,
		Passwd: neoPasswd,
		URL:    neoC.BoltURI,
		HTTP:   neoC.HttpURI,
	}
	gdb := neo.NewNeo(ncfg)
	err = gdb.Connect()
	if err != nil {
		logrus.Fatal(err)
	}
	err = gdb.Setup(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	/// Minio
	minioC, err := minio.SetupMinio(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer minioC.Terminate(ctx)
	configsBucketKey := minio.GetConfigsBucketKey()
	layersBucketKey := minio.GetLayersBucketKey()
	manifestsBucketKey := minio.GetManifestsBucketKey()
	layerCfg := minio.NewBucketConfig(layersBucketKey, minioC.HttpURI, minioC.ConsoleURI)
	layerCfg.Redirect = true
	bc := map[string]minio.BucketConfig{
		configsBucketKey:   minio.NewBucketConfig(configsBucketKey, minioC.HttpURI, minioC.ConsoleURI),
		layersBucketKey:    layerCfg,
		manifestsBucketKey: minio.NewBucketConfig(manifestsBucketKey, minioC.HttpURI, minioC.ConsoleURI),
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
	f, err := os.CreateTemp("", "sqlite-test")
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Start TestMain: %s", f.Name())
	defer os.Remove(f.Name())
	sq, err := sqlite.New(f.Name(), "")
	if err != nil {
		logrus.Fatal(err)
	}
	mag := magazine.Magazine{
		Minio:          min,
		Database:       gdb,
		UserManagement: sq,
	}
	h = NewHandlers("test", mag)
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}

func TeardownTest(bs utils.Bootstrap) {
	logrus.Info("> TeardownTest")
	h.mag.Minio.PreTearDown(bs)
	h.mag.Database.TeardownTest(bs)
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		logrus.Fatal(err)
	}
	err = h.mag.UserManagement.EmptyData(bs.EmptyAll)
	if err != nil {
		logrus.Fatal(err)
	}
}

func SetupTest(ctx context.Context, bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	return h.mag.SetupTest(ctx, bs)
}

/*
func SetupTestOLD(bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	logrus.Debug("> SetupTest")
	br = utils.NewBootstrapResult()

	for _, b := range bs.Bootstraps {
		sq := h.mag.SQlite
		min := h.mag.Minio
		switch b {
		default:
			err = fmt.Errorf("No Setup for '%s'", b)
			return br, err
		// Users/Profile/Platforms
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
		// Upstream
		case "upstreams:qnib/featuretest:arm64":
			err = sqlite.Setup_Upstream_FTarm(&sq, &br)
			if err != nil {
				return
			}
		case "upstreams:qnib/featuretest:amd64":
			err = sqlite.Setup_Upstream_FT_amd64(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Upstream_FT_amd64")
				return
			}
		// Layers
		case "layers:qnib/featuretest:amd64":
			err = sqlite.Setup_Layers_FT_amd64(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Layers_FT_amd64")
				return
			}
		case "layers:qnib/featuretest:arm64":
			err = sqlite.Setup_Layers_FTarm(&sq, &br)
			if err != nil {
				return
			}
		/// CONFIG
		case "config:qnib/featuretest:amd64":
			err = sqlite.Setup_Config_FT_amd64(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "sqlite.Setup_Config_FT_amd64")
				return
			}
			err = minio.Setup_Config_FT_amd64(&min, &br)
			if err != nil {
				err = errors.Wrap(err, "minio.Setup_Config_FT_amd64")
				return
			}
		case "config:qnib/featuretest:arm64":
			err = sqlite.Setup_Config_FTarm(&sq, &br)
			if err != nil {
				return
			}
			err = minio.Setup_Config_FT_arm64(&min, &br)
			if err != nil {
				err = errors.Wrap(err, "minio.Setup_Config_FT_arm64")
				return
			}
		/// Manifests
		case "manifest:qnib/featuretest:arm64":
			err = sqlite.Setup_Manifest_QNIB_FeatureTest_arm(&sq, &br)
			if err != nil {
				return
			}
		case "manifest:qnib/featuretest:amd64":
			err = sqlite.Setup_Manifest_QNIB_FeatureTest_amd(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifest_QNIB_FeatureTest_amd")
				return
			}
		/// ManifestLists
		case "manifestlist:qnib/featuretest:amd64":
			err = sqlite.Setup_Manifestlist_FTamd64(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifestlist_FTamd64")
				return
			}
		case "manifestlist:qnib/featuretest:arm64":
			err = sqlite.Setup_Manifestlist_FTarm(&sq, &br)
			if err != nil {
				return
			}
		/// Collections
		case "collection:qnib/featuretest:amd64":
			err = sqlite.Setup_Collection_QNIB_FeatureTest_amd64(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_amd64")
				return
			}
		case "collection:qnib/featuretest:generic":
			err = sqlite.Setup_Collection_QNIB_FeatureTest_Generic(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_Generic")
				return
			}

		case "collection:qnib/featuretest:arm64":
			err = sqlite.Setup_Collection_QNIB_FeatureTest_arm(&sq, &br)
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
				UploadUser: "test",
				UploadProfile: "generic",
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
				UploadUser: "test",
				UploadProfile: "generic",
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
*/
