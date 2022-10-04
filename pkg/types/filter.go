package types

type Filter struct {
	NeoID   int64  `json:"neo_id"`
	SqlID   int64  `json:"sql_id"`
	Label   string `json:"label"`
	Feature string `json:"feature"`
}
