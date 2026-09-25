package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pmarkun/teendns/internal/policy"
)

type Config struct {
	Listen      string           `json:"listen"`
	Upstream    string           `json:"upstream"`
	Certificate string           `json:"certificate"`
	PrivateKey  string           `json:"private_key"`
	MaxTTL      uint32           `json:"max_ttl"`
	Houses      []House          `json:"houses,omitempty"`
	Invitations []Invitation     `json:"invitations,omitempty"`
	Waitlist    []WaitlistEntry  `json:"waitlist,omitempty"`
	Profiles    []policy.Profile `json:"profiles"`
}

type House struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	AdminTokenHash string   `json:"admin_token_hash"`
	TimeZone       string   `json:"time_zone,omitempty"`
	Email          string   `json:"email,omitempty"`
	Emails         []string `json:"emails,omitempty"`
}

// Invitation is a single-use, expiring code an operator generates for a
// specific family. Email is redacted (set to "") once the invitation is
// consumed by registerHouse, since it survives from then on only as the
// House's own Email field.
type Invitation struct {
	CodeHash  string     `json:"code_hash"`
	Email     string     `json:"email,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

// WaitlistEntry records an email that asked for access before any house
// existed for it, so an operator can follow up with an invitation later.
type WaitlistEntry struct {
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
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
	for houseIndex := range cfg.Houses {
		house := &cfg.Houses[houseIndex]
		if len(house.Emails) == 0 && house.Email != "" {
			house.Emails = []string{house.Email}
		}
	}
	if err := ApplyHouseTimeZones(&cfg); err != nil {
		return Config{}, err
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

// ApplyHouseTimeZones gives each runtime profile its household time zone. The
// profile field is intentionally transient; the persistent source of truth is
// the owning house, so a house-level change applies consistently to all profiles.
func ApplyHouseTimeZones(cfg *Config) error {
	houseZones := make(map[string]string, len(cfg.Houses))
	for index := range cfg.Houses {
		house := &cfg.Houses[index]
		if house.TimeZone == "" {
			house.TimeZone = policy.DefaultTimeZone
		}
		if _, err := time.LoadLocation(house.TimeZone); err != nil {
			return fmt.Errorf("house %q time zone: %w", house.ID, err)
		}
		houseZones[house.ID] = house.TimeZone
	}
	for index := range cfg.Profiles {
		profile := &cfg.Profiles[index]
		profile.TimeZone = policy.DefaultTimeZone
		if zone, ok := houseZones[profile.HouseID]; ok {
			profile.TimeZone = zone
		}
	}
	return nil
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
