package utils

type BreadCrumb struct {
	Crumb  string `json:"crumb"`
	Active int    `json:"active"`
	Link   string `json:"link"`
}
