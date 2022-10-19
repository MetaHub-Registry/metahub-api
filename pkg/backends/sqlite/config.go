package sqlite

type Config struct {
	ID        int64  `json:"id"`
	Digest    string `json:"digest"`
	Label     string `json:"label"`
	MediaType string `json:"media_type"`
	Size      int    `json:"size"`
}

/*
func (c Config) GetConfigByConfigID(cfgID int64) (cfg types.Config, err error) {
	q := `SELECT
		configs.digest AS configDigest,
		configs.id AS configID,
		configs.size AS configSize,
		configs.mediaType AS configMediaType
	FROM configs WHERE configs.id=%d;`
	query := fmt.Sprintf(q, cfgID)
	rows, err := db.Query(query)
	if err != nil {
		logrus.Error(err)
	}
	cfg = types.Config{
		SqlID: cfgID,
	}
	for rows.Next() {
		res := cfgRes{}
		err = rows.Scan(
			&res.ConfigDigest,
			&res.ConfigID,
			&res.ConfigSize,
			&res.ConfigMediaType,
		)
		cfg.Digest = res.ConfigDigest
		cfg.Size = res.ConfigSize
		cfg.MediaType = res.ConfigMediaType
	}
	return
}
*/
