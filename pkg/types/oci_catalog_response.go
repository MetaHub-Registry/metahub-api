package types

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
)

type CatalogResponse struct {
	Repos []string `json:"repositories"`
}

func (r CatalogResponse) WriteWWWHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func (r CatalogResponse) WriteWWWBody(w http.ResponseWriter) {
	b, err := r.ToBytes()
	if err != nil {
		logrus.Errorf("failed to marshal manifest: %v", err)
		return
	}
	w.Write(b)
}

func (r CatalogResponse) ToBytes() (data []byte, err error) {
	return json.Marshal(r)
}
