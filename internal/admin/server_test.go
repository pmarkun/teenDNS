package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/gateway"
	"github.com/pmarkun/teendns/internal/policy"
)

func TestUpdateProfilePersistsAndActivatesImmediately(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := policy.NewManager(cfg.Profiles)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(path, cfg, manager, gateway.NewEventBuffer(), "dns.teendns.test", "secret")
	if err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"label":"Casa","default_action":"allow","rules":[{"domain":"bets.test","include_subdomains":true,"action":"block","category":"gambling","reason":"Acordo familiar"}]}`)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/home", body)
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	active, ok := manager.Profile("p-home.dns.teendns.test")
	if !ok {
		t.Fatal("updated profile is not active")
	}
	decision, err := policy.Decide(active, "promo.bets.test")
	if err != nil || decision.Action != policy.ActionBlock {
		t.Fatalf("policy was not replaced immediately: %+v, %v", decision, err)
	}
	persisted, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Profiles[0].Version != 2 || len(persisted.Profiles[0].Rules) != 1 {
		t.Fatalf("unexpected persisted profile: %+v", persisted.Profiles[0])
	}
}

func TestCreateProfileGeneratesOpaqueEndpoint(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), "dns.teendns.test", "secret")

	request := httptest.NewRequest(http.MethodPost, "/api/v1/profiles", bytes.NewBufferString(`{"label":"Estudos"}`))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	var created policy.Profile
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Label != "Estudos" || created.Hostname == "" || created.ID == "" {
		t.Fatalf("unexpected created profile: %+v", created)
	}
	if created.Hostname == "p-estudos.dns.teendns.test" {
		t.Fatal("hostname must not expose the profile label")
	}
}

func TestAdminRequiresBearerToken(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), "dns.teendns.test", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestMergeGroupsRestoresServerDefaults(t *testing.T) {
	existing := []policy.RuleGroup{{
		ID:             "gambling",
		Name:           "Apostas",
		Action:         policy.ActionBlock,
		Domains:        []string{"custom.test"},
		DefaultDomains: []string{"one.test", "two.test"},
		DomainSource:   "catalog.txt",
		Customized:     true,
	}}
	incoming := []policy.RuleGroup{{
		ID:         "gambling",
		Name:       "Apostas",
		Action:     policy.ActionBlock,
		Domains:    []string{"ignored.test"},
		Customized: false,
	}}
	groups, err := mergeGroups(existing, incoming)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups[0].Domains) != 2 || groups[0].Domains[0] != "one.test" || groups[0].DomainSource != "catalog.txt" {
		t.Fatalf("defaults were not restored: %+v", groups[0])
	}
}

func testConfig() config.Config {
	return config.Config{
		Listen:      ":853",
		Upstream:    "unbound:5353",
		Certificate: "cert.pem",
		PrivateKey:  "key.pem",
		MaxTTL:      300,
		Profiles: []policy.Profile{{
			ID:            "home",
			Label:         "Casa",
			Hostname:      "p-home.dns.teendns.test",
			DefaultAction: policy.ActionAllow,
			Version:       1,
			Rules:         []policy.Rule{},
		}},
	}
}
