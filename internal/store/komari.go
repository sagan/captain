package store

// KomariSettings is retained only for decoding and clearing retired settings.
type KomariSettings struct {
	Enabled  bool   `json:"enabled"`
	Server   string `json:"server"`
	Key      string `json:"key"` // auto-discovery key; write-only in the API
	Interval int    `json:"interval"`
}

const SettingKomari = "komari"
