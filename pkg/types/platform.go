package types

type Platform struct {
	NeoID    int64  `json:"neo_id"`
	SqlID    int64  `json:"sql_id"`
	Arch     string `json:"architecture"`
	OS       string `json:"os"`
	Variant  string `json:"variant"`
	Features string `json:"features"`
}
