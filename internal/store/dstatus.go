package store

// DStatusSettings is retained only for decoding and clearing retired settings.
type DStatusSettings struct {
	Enabled  bool   `json:"enabled"`
	Mode     string `json:"mode"`   // "" or "passive", or "active"
	Listen   string `json:"listen"` // passive: host:port; "" = :9999
	Key      string `json:"key"`    // write-only in the API
	Server   string `json:"server"` // active: the panel's base URL
	Interval int    `json:"interval"`
}

const SettingDStatus = "dstatus"
