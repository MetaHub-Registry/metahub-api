package types

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
)

type TagResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (t TagResponse) WriteWWWHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func (t TagResponse) WriteWWWBody(w http.ResponseWriter) {
	b, err := t.ToBytes()
	if err != nil {
		logrus.Errorf("failed to marshal manifest: %v", err)
		return
	}
	w.Write(b)
}

func (t TagResponse) ToBytes() (data []byte, err error) {
	return json.Marshal(t)
}
