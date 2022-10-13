package handlers

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/sirupsen/logrus"
)

func (h Handlers) ManifestHandlerGet(w http.ResponseWriter, r *http.Request) {
	logrus.Debug(">> ManifestHandlerGet")
	dumpedBody, _ := httputil.DumpRequest(r, true)
	fmt.Printf("%s", dumpedBody)

	return
}
