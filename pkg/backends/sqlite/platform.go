package sqlite

type Platform struct {
	ID       int64  `json:"id"`
	Arch     string `json:"architecture"`
	OS       string `json:"os"`
	Variant  string `json:"variant"`
	Features string `json:"features"`
}
