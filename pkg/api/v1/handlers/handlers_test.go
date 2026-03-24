package handlers

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	neoDb     = "test"
	neoUser   = "neo4j"
	neoPasswd = "test"
	svcName   = "api-test"
)

var (
	h               Handlers
	regHttpURI      string
	jeagerHttpURI   string
	jeagerThriftURI string
)

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	/// Neo4j
	neoP := os.Getenv("NEO4J_PORTS")
	neoC, err := neo.SetupNeo4j(ctx, neoP)
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
	/// Registry
	regC, err := upstream.SetupRegistry(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer regC.Terminate(ctx)
	regHttpURI = regC.HttpURI
	//jeager
	jeagerC, err := tracing.SetupJeager(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer jeagerC.Terminate(ctx)
	jeagerHttpURI = jeagerC.HttpURI
	jeagerThriftURI = jeagerC.ThriftURI
	/// Minio
	minioP := os.Getenv("MINIO_PORTS")
	minioC, err := minio.SetupMinio(ctx, minioP)
	if err != nil {
		logrus.Fatal(err)
	}
	defer minioC.Terminate(ctx)
	blobBucketKey := minio.GetBlobBucketKey()
	manifestsBucketKey := minio.GetManifestsBucketKey()
	blobCfg := minio.NewBucketConfig(blobBucketKey, minioC.HttpURI, minioC.ConsoleURI)
	blobCfg.Redirect = true
	bc := map[string]minio.BucketConfig{
		blobBucketKey:      blobCfg,
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
	trEnd := fmt.Sprintf("http://%s/api/traces", jeagerC.ThriftURI)
	logrus.Infof("Tracing endpoint '%s': %s", svcName, trEnd)
	tr, err := tracing.InitTracer(svcName, trEnd)
	if err != nil {
		logrus.Fatalf("Could not initialize jaeger tracer: %s", err.Error())
	}
	trCfg := magazine.TraceCfg{
		Enabled:  true,
		Endpoint: jeagerC.ThriftURI,
		Tracer:   tr,
	}
	mag := magazine.Magazine{
		Minio:          min,
		Database:       gdb,
		UserManagement: sq,
		Config: magazine.MagConfig{
			Tracing: trCfg,
		},
	}
	logrus.Infof("Registry: %s", regC.HttpURI)
	mag.Config.Args = map[string]string{
		"TEST_REGISTRY":      regC.HttpURI,
		"TEST_JAEGER_HTTP":   jeagerC.HttpURI,
		"TEST_JAEGER_THRIFT": jeagerC.ThriftURI,
		"registries":         regC.HttpURI,
	}
	logrus.Infof("Jaeger HTTP: http://%s", jeagerC.HttpURI)
	h = NewHandlers("test", mag, nil)
	code := m.Run()
	switch code {
	case 0:
		logrus.Info("Tests OK")
	default:
		logrus.Infof("Tests FAILED with code %d", code)
	}
}

func TeardownTest(bs utils.Bootstrap, br utils.BootstrapResult) {
	logrus.Info("> TeardownTest")
	jh, ok := h.mag.Config.Args["TEST_JAEGER_HTTP"]
	if ok {
		logrus.Infof("Jaeger HTTP: http://%s", jh)
	}

	h.mag.Minio.PreTearDown(bs)
	h.mag.Database.TeardownTest(bs, br)
	if bs.PostTestSleep > 0 {
		sqfile, err := br.Get("sqfile")
		if err != nil {
			logrus.Error("No 'sqfile' in BootstrapResult")
		} else {
			logrus.Infof("SQLite File: %s", sqfile)
			sqbC, err := sqlite.SetupSQBrowser(context.Background(), sqfile, "")
			if err != nil {
				logrus.Fatal(err)
			}
			defer func() {
				if err := sqbC.Terminate(context.Background()); err != nil {
					logrus.Fatal(err)
				}
			}()
			logrus.Infof("SQLite Browser at: http://%s", sqbC.HttpURI)
		}
		logrus.Infof("Sleep %d seconds for inspection...", bs.PostTestSleep)
		time.Sleep(time.Duration(bs.PostTestSleep) * time.Second)
	}
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Info(" > CopySqlite")
	if bs.CopySqlite != "" {
		logrus.Warnf("Backup sqlite to %s", bs.CopySqlite)
		err = h.mag.UserManagement.Backup(bs.CopySqlite)
		if err != nil {
			logrus.Fatal(err)
		}
	}
	logrus.Info(" > EmptyData")
	err = h.mag.UserManagement.EmptyData(bs.EmptyAll)
	if err != nil {
		logrus.Fatal(err)
	}
}

func SetupTest(ctx context.Context, bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	return h.mag.SetupTest(ctx, bs)
}
