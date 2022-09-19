package handlers

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

func (h Handlers) CollectionGetAll(w http.ResponseWriter, r *http.Request) {
	logrus.Info("CollectionGetAll")
	cols, err := h.models.Collection.GetAll()
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	for idx, col := range cols {
		manis, err := h.models.Manifest.GetByCollectionId(col.ID)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
			return
		}
		col.ManifestCount = len(manis)
		for _, mani := range manis {
			key := fmt.Sprintf("%s/%s:%s", mani.Upstream.Reg, mani.Upstream.Repo, mani.Upstream.Tag)
			col.Manifests[key] = mani

		}
		cols[idx] = col
	}
	_ = h.writeJSON(w, http.StatusOK, envelope{"collections": cols})
}
