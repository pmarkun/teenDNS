package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/gateway"
	"github.com/pmarkun/teendns/internal/pairing"
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
	server, err := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), "dns.teendns.test", "secret")
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
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), "dns.teendns.test", "secret")

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

func TestRegisterHouseConsumesInvitationAndScopesAdminToken(t *testing.T) {
	t.Setenv("TEENDNS_INVITATION_CODES", "convite-unico")
	t.Setenv("TEENDNS_CATALOG_DIR", filepath.Join("..", "..", "catalog", "v1"))
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), "dns.teendns.test", "operator-secret")

	register := func() *httptest.ResponseRecorder {
		body := bytes.NewBufferString(`{"invitation_code":"convite-unico","house_name":"Casa Silva","profile_name":"Lia","preset":"explorando"}`)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/houses", body)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		return response
	}

	response := register()
	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	var created registrationResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.House.Name != "Casa Silva" || created.AdminToken == "" || created.Profile.HouseID != created.House.ID {
		t.Fatalf("unexpected registration: %+v", created)
	}
	if len(created.Profile.Groups) != 15 || groupAction(created.Profile.Groups, "service-instagram") != policy.ActionObserve {
		t.Fatalf("expected exploring preset groups, got %+v", created.Profile.Groups)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	request.Header.Set("Authorization", "Bearer "+created.AdminToken)
	profilesResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(profilesResponse, request)
	if profilesResponse.Code != http.StatusOK {
		t.Fatalf("expected scoped token to work, got %d: %s", profilesResponse.Code, profilesResponse.Body.String())
	}
	var result struct {
		Profiles []policy.Profile `json:"profiles"`
	}
	if err := json.NewDecoder(profilesResponse.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Profiles) != 1 || result.Profiles[0].HouseID != created.House.ID {
		t.Fatalf("house token leaked profiles: %+v", result.Profiles)
	}
	legacyRequest := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/home", nil)
	legacyRequest.Header.Set("Authorization", "Bearer "+created.AdminToken)
	legacyResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(legacyResponse, legacyRequest)
	if legacyResponse.Code != http.StatusNotFound {
		t.Fatalf("expected another house profile to stay hidden, got %d", legacyResponse.Code)
	}

	reused := register()
	if reused.Code != http.StatusForbidden {
		t.Fatalf("expected reused invitation to fail, got %d: %s", reused.Code, reused.Body.String())
	}
}

func TestPresetGroupsChooseActionsWithoutStoringAge(t *testing.T) {
	catalogDir := filepath.Join("..", "..", "catalog", "v1")
	tests := []struct {
		id                        string
		adult, instagram, discord policy.Action
	}{
		{id: "acompanhado", adult: policy.ActionBlock, instagram: policy.ActionBlock, discord: policy.ActionObserve},
		{id: "explorando", adult: policy.ActionBlock, instagram: policy.ActionObserve, discord: policy.ActionObserve},
		{id: "autonomia-guiada", adult: policy.ActionBlock, instagram: policy.ActionAllow, discord: policy.ActionAllow},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			groups, err := presetGroups(test.id, catalogDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(groups) != 15 || groupAction(groups, "gambling-br") != policy.ActionBlock || groupAction(groups, "adult") != test.adult || groupAction(groups, "service-instagram") != test.instagram || groupAction(groups, "service-discord") != test.discord {
				t.Fatalf("unexpected preset: %+v", groups)
			}
		})
	}
	if _, err := presetGroups("inventado", catalogDir); err == nil {
		t.Fatal("expected unknown preset error")
	}
}

func groupAction(groups []policy.RuleGroup, id string) policy.Action {
	for _, group := range groups {
		if group.ID == id {
			return group.Action
		}
	}
	return ""
}

func TestAdminRequiresBearerToken(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), "dns.teendns.test", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestCloneConfigPreservesEmptyDomainLists(t *testing.T) {
	cfg := testConfig()
	cfg.Profiles[0].Groups = []policy.RuleGroup{{
		ID:             "adult",
		Name:           "Conteúdo adulto",
		Action:         policy.ActionBlock,
		Domains:        []string{},
		DefaultDomains: []string{},
	}}

	cloned := cloneConfig(cfg)
	group := cloned.Profiles[0].Groups[0]
	if group.Domains == nil || group.DefaultDomains == nil {
		t.Fatalf("expected empty domain lists, got %+v", group)
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

func TestPairingSessionReturnsOnlyProtectedCategoryNamesAndReasons(t *testing.T) {
	cfg := testConfig()
	cfg.Profiles[0].Label = "Casa"
	cfg.Profiles[0].Groups = []policy.RuleGroup{
		{ID: "gambling", Name: "Apostas", Action: policy.ActionBlock, Reason: "Apostas envolvem dinheiro real", Domains: []string{"secret.bet"}},
		{ID: "tracking", Name: "Rastreamento", Action: policy.ActionObserve, Reason: "Só observar", Domains: []string{"tracker.test"}},
	}
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	pairings := testPairing()
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), pairings, "dns.teendns.test", "secret")

	create := httptest.NewRequest(http.MethodPost, "/api/v1/pairing/challenges", nil)
	created := httptest.NewRecorder()
	server.Handler().ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", created.Code, created.Body.String())
	}
	var challenge pairing.Challenge
	if err := json.NewDecoder(created.Body).Decode(&challenge); err != nil {
		t.Fatal(err)
	}
	if !pairings.Observe("home", challenge.DNSName) {
		t.Fatal("could not pair challenge")
	}

	statusResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(statusResponse, httptest.NewRequest(http.MethodGet, "/api/v1/pairing/challenges/"+challenge.ID, nil))
	var status pairing.Status
	if err := json.NewDecoder(statusResponse.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/youth/profile", nil)
	request.Header.Set("Authorization", "Bearer "+status.SessionToken)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, `"name":"Apostas"`) || !strings.Contains(body, `"reason":"Apostas envolvem dinheiro real"`) {
		t.Fatalf("protected category missing: %s", body)
	}
	for _, hidden := range []string{"secret.bet", "Rastreamento", "tracker.test", "hostname", "profile_id"} {
		if strings.Contains(body, hidden) {
			t.Fatalf("youth response leaked %q: %s", hidden, body)
		}
	}
}

func testPairing() *pairing.Manager {
	return pairing.NewManager("pair.teendns.test", time.Minute, time.Hour)
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
