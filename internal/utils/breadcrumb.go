package utils

type BreadCrumb struct {
	Crumb  string `json:"crumb"`
	Active int    `json:"active"`
	Link   string `json:"link"`
	Tag    string `json:"tag"`
	Repo   string `json:"repo"`
	IDX    int    `json:"idx"`
}
