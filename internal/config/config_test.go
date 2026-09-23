package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmarkun/teendns/internal/policy"
)

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
