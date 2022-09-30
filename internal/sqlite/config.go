package sqlite

type Config struct {
	ID        int64  `json:"id"`
	Digest    string `json:"digest"`
	Label     string `json:"label"`
	MediaType string `json:"media_type"`
	Size      int    `json:"size"`
}
