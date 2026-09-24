package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pmarkun/teendns/internal/policy"
)

type Config struct {
	Listen             string           `json:"listen"`
	Upstream           string           `json:"upstream"`
	Certificate        string           `json:"certificate"`
	PrivateKey         string           `json:"private_key"`
	MaxTTL             uint32           `json:"max_ttl"`
	Houses             []House          `json:"houses,omitempty"`
	UsedInvitationKeys []string         `json:"used_invitation_keys,omitempty"`
	Profiles           []policy.Profile `json:"profiles"`
}

type House struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	AdminTokenHash string `json:"admin_token_hash"`
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
	for profileIndex := range cfg.Profiles {
		for groupIndex := range cfg.Profiles[profileIndex].Groups {
			group := &cfg.Profiles[profileIndex].Groups[groupIndex]
			if group.Domains == nil {
				group.Domains = []string{}
			}
			if group.DomainSource == "" {
				if group.DefaultDomains == nil {
					group.DefaultDomains = append([]string{}, group.Domains...)
				}
				continue
			}
			defaults, err := loadDomains(group.DomainSource)
			if err != nil {
				return Config{}, fmt.Errorf("profile %q group %q source: %w", cfg.Profiles[profileIndex].ID, group.ID, err)
			}
			group.DefaultDomains = defaults
			if !group.Customized {
				group.Domains = append([]string{}, defaults...)
			}
		}
	}
	if _, err := policy.NewStore(cfg.Profiles); err != nil {
		return Config{}, fmt.Errorf("profiles: %w", err)
	}
	return cfg, nil
}

func loadDomains(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	domains := make([]string, 0)
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		domains = append(domains, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return domains, nil
}

// LoadDomains reads one normalized catalog file for provisioning code. Runtime
// config loading uses the same parser so presets and persisted profiles cannot
// disagree about comments, duplicates or casing.
func LoadDomains(path string) ([]string, error) {
	return loadDomains(path)
}
