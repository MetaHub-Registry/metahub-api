package magazine

import "github.com/metahub-registry/metahub-api/pkg/types"

func (mag Magazine) Tags_GetList(req types.Request) (res types.TagResponse, err error) {
	// Fetch Layer from SQLite
	res, err = mag.SQlite.Collection.GetTagsListByRepo(req.Repo)
	if err != nil {
		return
	}
	return
}
