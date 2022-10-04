package types

type Collection struct {
	NeoID         int64          `json:"neo_id"`
	SqlID         int64          `json:"sql_id"`
	Repo          string         `json:"repo"`
	Tag           string         `json:"tag"`
	ManifestLists []ManifestList `json:"manifest_list"`
}
