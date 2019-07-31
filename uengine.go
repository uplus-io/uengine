package uengine

type UEngineConfig struct {
	Engine     string   `json:"engine" yaml:"engine"`
	Meta       string   `json:"meta" yaml:"meta"`
	Wal        string   `json:"wal" yaml:"wal"`
	Partitions []string `json:"partitions" yaml:"partitions"`
}