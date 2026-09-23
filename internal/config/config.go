package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pmarkun/teendns/internal/policy"
)

type Config struct {
	Listen      string           `json:"listen"`
	Upstream    string           `json:"upstream"`
	Certificate string           `json:"certificate"`
	PrivateKey  string           `json:"private_key"`
	MaxTTL      uint32           `json:"max_ttl"`
	Profiles    []policy.Profile `json:"profiles"`
}

func Load(path string) (Config, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(contents, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Listen == "" || cfg.Upstream == "" {
		return Config{}, fmt.Errorf("listen and upstream are required")
	}
	if cfg.Certificate == "" || cfg.PrivateKey == "" {
		return Config{}, fmt.Errorf("certificate and private_key are required")
	}
	if cfg.MaxTTL == 0 {
		cfg.MaxTTL = 300
	}
	if _, err := policy.NewStore(cfg.Profiles); err != nil {
		return Config{}, fmt.Errorf("profiles: %w", err)
	}
	return cfg, nil
}
