package magazine

import (
	"sort"

	"github.com/chrispappas/golang-generics-set/set"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func (mag Magazine) Collections_GetCatalog(req types.Request) (res types.CatalogResponse, err error) {
	// Fetch Layer from SQLite
	cols, err := mag.SQlite.Collection.GetAll()
	if err != nil {
		return
	}
	res = types.CatalogResponse{
		Repos: []string{},
	}
	r := set.FromSlice([]string{})
	for _, col := range cols {
		r.Add(col.Repo)
	}
	repos := r.Values()
	sort.Strings(repos)
	// TODO: add pagination
	for _, repo := range repos {
		res.Repos = append(res.Repos, repo)
	}
	return
}
