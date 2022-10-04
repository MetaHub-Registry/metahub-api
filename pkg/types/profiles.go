package types

type Profile struct {
	NeoID int64  `json:"neo_id"`
	SqlID int64  `json:"sql_id"`
	Name  string `json:"name"`
}
