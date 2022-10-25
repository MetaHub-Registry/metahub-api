package types

import "github.com/opencontainers/go-digest"

type Upstream struct {
	NeoID  int64         `json:"neo_id"`
	SqlID  int64         `json:"sql_id"`
	Reg    string        `json:"reg"`
	Repo   string        `json:"repo"`
	Tag    string        `json:"tag"`
	Digest digest.Digest `json:"digest"`
}
