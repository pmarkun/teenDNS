package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmarkun/teendns/internal/policy"
)

func TestLoadExpandsRuleGroupDomainSource(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "domains.txt")
	if err := os.WriteFile(source, []byte("one.test\n# comment\ntwo.test\none.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "gateway.json")
	contents := `{"listen":":853","upstream":"dns:53","certificate":"cert","private_key":"key","profiles":[{"id":"home","hostname":"home.test","default_action":"allow","groups":[{"id":"default","name":"Padrão","action":"block","domains":[],"domain_source":"` + source + `"}]}]}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	group := cfg.Profiles[0].Groups[0]
	if len(group.Domains) != 2 || len(group.DefaultDomains) != 2 || group.Domains[1] != "two.test" {
		t.Fatalf("unexpected expanded group: %+v", group)
	}
}

func TestWriteAtomicRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	want := Config{
		Listen:      ":853",
		Upstream:    "127.0.0.1:5353",
		Certificate: "server.pem",
		PrivateKey:  "server-key.pem",
		MaxTTL:      120,
		Profiles: []policy.Profile{
			{ID: "ana", Hostname: "p-ana.dns.teendns.test", DefaultAction: policy.ActionAllow, Version: 1},
		},
	}
	if err := WriteAtomic(path, want, 0o640); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("expected permissions 0640, got %o", info.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxTTL != want.MaxTTL || len(got.Profiles) != 1 || got.Profiles[0].ID != "ana" {
		t.Fatalf("unexpected round trip config: %+v", got)
	}
}

func TestLoadMigratesHousePrimaryEmailIntoEmails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	contents := []byte(`{
  "listen": ":853",
  "upstream": "127.0.0.1:5353",
  "certificate": "server.pem",
  "private_key": "server-key.pem",
  "profiles": [],
  "houses": [{"id":"house-1","name":"Casa Silva","admin_token_hash":"abc","email":"responsavel@example.com"}]
}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Houses) != 1 || len(cfg.Houses[0].Emails) != 1 || cfg.Houses[0].Emails[0] != "responsavel@example.com" {
		t.Fatalf("expected primary email migrated into Emails, got %+v", cfg.Houses[0])
	}
}

func TestLoadDefaultsAndAppliesHouseTimeZone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	contents := []byte(`{
  "listen": ":853",
  "upstream": "127.0.0.1:5353",
  "certificate": "server.pem",
  "private_key": "server-key.pem",
  "profiles": [
    {"id":"ana","house_id":"house-1","hostname":"ana.test","default_action":"allow"},
    {"id":"local","hostname":"local.test","default_action":"allow"}
  ],
  "houses": [{"id":"house-1","name":"Casa","admin_token_hash":"abc","time_zone":"America/Manaus"}]
}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles[0].TimeZone != "America/Manaus" {
		t.Fatalf("expected profile to inherit house time zone, got %q", cfg.Profiles[0].TimeZone)
	}
	if cfg.Profiles[1].TimeZone != policy.DefaultTimeZone {
		t.Fatalf("expected unowned profile to use default time zone, got %q", cfg.Profiles[1].TimeZone)
	}
	if cfg.Houses[0].TimeZone != "America/Manaus" {
		t.Fatalf("unexpected persisted house time zone: %q", cfg.Houses[0].TimeZone)
	}
}

func TestLoadMigratesMissingHouseTimeZoneToDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	contents := []byte(`{
  "listen": ":853",
  "upstream": "127.0.0.1:5353",
  "certificate": "server.pem",
  "private_key": "server-key.pem",
  "profiles": [],
  "houses": [{"id":"house-1","name":"Casa","admin_token_hash":"abc"}]
}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Houses[0].TimeZone != policy.DefaultTimeZone {
		t.Fatalf("expected default house time zone, got %q", cfg.Houses[0].TimeZone)
	}
}

func TestLoadRejectsInvalidHouseTimeZone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	contents := []byte(`{
  "listen": ":853",
  "upstream": "127.0.0.1:5353",
  "certificate": "server.pem",
  "private_key": "server-key.pem",
  "profiles": [],
  "houses": [{"id":"house-1","name":"Casa","admin_token_hash":"abc","time_zone":"Mars/Olympus"}]
}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid time zone to fail")
	}
}

func TestLoadAppliesDefaultTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	contents := []byte(`{
  "listen": ":853",
  "upstream": "127.0.0.1:5353",
  "certificate": "server.pem",
  "private_key": "server-key.pem",
  "profiles": []
}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxTTL != 300 {
		t.Fatalf("expected default TTL 300, got %d", cfg.MaxTTL)
	}
}

func TestLoadNormalizesEmptyRuleGroupDomains(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	contents := []byte(`{
  "listen": ":853",
  "upstream": "127.0.0.1:5353",
  "certificate": "server.pem",
  "private_key": "server-key.pem",
  "profiles": [{
    "id": "ana",
    "hostname": "p-ana.dns.teendns.test",
    "default_action": "allow",
    "groups": [{"id":"adult","name":"Conteúdo adulto","action":"block"}]
  }]
}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	group := cfg.Profiles[0].Groups[0]
	if group.Domains == nil || group.DefaultDomains == nil {
		t.Fatalf("expected empty domain lists, got %+v", group)
	}
}

func TestLoadRejectsInvalidProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	contents := []byte(`{
  "listen": ":853",
  "upstream": "127.0.0.1:5353",
  "certificate": "server.pem",
  "private_key": "server-key.pem",
  "profiles": [{"id":"ana","hostname":"invalid host","default_action":"allow"}]
}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid profile error")
	}
}
