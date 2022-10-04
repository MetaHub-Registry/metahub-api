package types

type ManifestList struct {
	NeoID     int64      `json:"neo_id"`
	SqlID     int64      `json:"sql_id"`
	Digest    string     `json:"digest"`
	MediaType string     `json:"media_type"`
	Size      int64      `json:"size"`
	Manifests []Manifest `json:"manifests"`
}
