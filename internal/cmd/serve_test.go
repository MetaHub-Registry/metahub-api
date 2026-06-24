package cmd

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/metahub-registry/metahub-api/pkg/metrics"
)

func newServeTestCmd() *cobra.Command {
	c := &cobra.Command{Use: "serve"}
	c.Flags().Int(flagMetricsPort, 9090, "metrics port")
	return c
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to grab a free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func TestStartMetricsServerDisabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset(); metrics.SetEnabled(true) })
	viper.Set("metrics.enabled", false)

	// Should return promptly without starting a listener and leave metrics off.
	startMetricsServer(newServeTestCmd())
	if metrics.Enabled() {
		t.Fatal("metrics enabled after startMetricsServer with metrics.enabled=false")
	}
}

func TestStartMetricsServerEnabledServesMetrics(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset(); metrics.SetEnabled(true) })
	viper.Set("metrics.enabled", true)

	port := freePort(t)
	cmd := newServeTestCmd()
	if err := cmd.Flags().Set(flagMetricsPort, strconv.Itoa(port)); err != nil {
		t.Fatalf("set metrics-port flag: %v", err)
	}

	startMetricsServer(cmd)

	url := fmt.Sprintf("http://127.0.0.1:%d/metrics", port)
	var status int
	for i := 0; i < 100; i++ {
		resp, err := http.Get(url)
		if err == nil {
			status = resp.StatusCode
			_ = resp.Body.Close()
			if status == http.StatusOK {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want 200 (metrics server did not come up)", status)
	}
}
