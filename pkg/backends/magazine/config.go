package magazine

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func (mag Magazine) ConfigBlob_GetByHTTPRequest(r *http.Request) (cfg types.Config, found bool, err error) {
	ctx := context.Background()
	req := types.NewRequest(r)
	b, err := mag.Minio.ConfigGetBytes(ctx, req.Digest)
	switch {
	case err != nil && err.Error() == "The specified key does not exist.":
		// found is false
		err = nil
		return
	case err != nil:
		logrus.Warn(err)
		return
	}
	// No error - off we go
	found = true
	cfg, err = types.NewConfigFromBytes(b)
	if err != nil {
		logrus.Error(err)
	}

	return
}
