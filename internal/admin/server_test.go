package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/gateway"
	"github.com/pmarkun/teendns/internal/magiclink"
	"github.com/pmarkun/teendns/internal/mail"
	"github.com/pmarkun/teendns/internal/pairing"
	"github.com/pmarkun/teendns/internal/policy"
)

func TestUpdateProfilePersistsAndActivatesImmediately(t *testing.T) {
	cfg := testConfig()
	cfg.Profiles[0].Groups = []policy.RuleGroup{{ID: "games", Name: "Jogos", Action: policy.ActionBlock, Domains: []string{"games.test"}}}
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := policy.NewManager(cfg.Profiles)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "secret", testMailer())
	if err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"label":"Casa","default_action":"allow","rules":[{"domain":"bets.test","include_subdomains":true,"action":"block","category":"gambling","reason":"Acordo familiar"}],"groups":[{"id":"games","name":"Jogos","action":"block","domains":["games.test"],"schedules":[{"id":"after-school","label":"Depois da escola","days":[1,2,3,4,5],"start":"16:00","end":"17:00","action":"allow"}]}],"pauses":[{"id":"sleep","label":"Dormir","days":[0,1,2,3,4,5,6],"start":"22:00","end":"07:00"}]}`)
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
	if len(persisted.Profiles[0].Pauses) != 1 || persisted.Profiles[0].Pauses[0].Label != "Dormir" || len(persisted.Profiles[0].Groups[0].Schedules) != 1 {
		t.Fatalf("scheduled windows were not persisted: %+v", persisted.Profiles[0])
	}
	active, ok = manager.Profile("p-home.dns.teendns.test")
	if !ok || len(active.Pauses) != 1 || active.Location == nil {
		t.Fatalf("scheduled policy was not activated with its time zone: %+v", active)
	}
}

func TestCreateProfileGeneratesOpaqueEndpoint(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "secret", testMailer())

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
	t.Setenv("TEENDNS_CATALOG_DIR", filepath.Join("..", "..", "catalog", "v1"))
	cfg := testConfig()
	cfg.Invitations = []config.Invitation{{
		CodeHash:  tokenHash("convite-unico"),
		Email:     "responsavel@example.com",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}}
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

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

	snapshot := server.Snapshot()
	if len(snapshot.Houses) != 1 || snapshot.Houses[0].Email != "responsavel@example.com" {
		t.Fatalf("expected invitation email copied to house, got %+v", snapshot.Houses)
	}
	if len(snapshot.Houses[0].Emails) != 1 || snapshot.Houses[0].Emails[0] != "responsavel@example.com" {
		t.Fatalf("expected invitation email copied to house admin emails, got %+v", snapshot.Houses[0].Emails)
	}
	if len(snapshot.Invitations) != 1 || snapshot.Invitations[0].Email != "" || snapshot.Invitations[0].UsedAt == nil {
		t.Fatalf("expected invitation redacted and marked used, got %+v", snapshot.Invitations)
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

func TestRegisterHouseRejectsExpiredInvitation(t *testing.T) {
	t.Setenv("TEENDNS_CATALOG_DIR", filepath.Join("..", "..", "catalog", "v1"))
	cfg := testConfig()
	cfg.Invitations = []config.Invitation{{
		CodeHash:  tokenHash("convite-vencido"),
		Email:     "responsavel@example.com",
		CreatedAt: time.Now().Add(-8 * 24 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}}
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	body := bytes.NewBufferString(`{"invitation_code":"convite-vencido","house_name":"Casa Silva","profile_name":"Lia","preset":"explorando"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/houses", body)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected expired invitation to be rejected with 403, got %d: %s", response.Code, response.Body.String())
	}
}

type recordingMailer struct {
	sent []struct{ to, subject, html string }
	fail bool
}

func (m *recordingMailer) Send(to, subject, html string) error {
	if m.fail {
		return errors.New("send failed")
	}
	m.sent = append(m.sent, struct{ to, subject, html string }{to, subject, html})
	return nil
}

func TestCreateInvitationRequiresOperatorToken(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	houseToken, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := server.update(func(cfg *config.Config) error {
		cfg.Houses = append(cfg.Houses, config.House{ID: "house-1", Name: "Casa", AdminTokenHash: tokenHash(houseToken)})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"email":"responsavel@example.com"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/invitations", body)
	request.Header.Set("Authorization", "Bearer "+houseToken)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected house token to be rejected with 403, got %d: %s", response.Code, response.Body.String())
	}
}

func TestCreateInvitationSendsEmailAndPersistsRecord(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	mailer := &recordingMailer{}
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", mailer)

	body := bytes.NewBufferString(`{"email":"responsavel@example.com"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/invitations", body)
	request.Header.Set("Authorization", "Bearer operator-secret")
	request.Host = "teendns.lab.markun.com.br"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	var created invitationResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Email != "responsavel@example.com" || created.Code == "" || !created.EmailSent {
		t.Fatalf("unexpected invitation response: %+v", created)
	}
	if created.Link != "https://teendns.lab.markun.com.br/comecar?convite="+created.Code {
		t.Fatalf("unexpected invitation link: %q", created.Link)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].to != "responsavel@example.com" {
		t.Fatalf("expected exactly one email sent to the invited address, got %+v", mailer.sent)
	}

	snapshot := server.Snapshot()
	if len(snapshot.Invitations) != 1 || snapshot.Invitations[0].UsedAt != nil {
		t.Fatalf("expected one unused invitation persisted, got %+v", snapshot.Invitations)
	}
	if snapshot.Invitations[0].CodeHash != tokenHash(created.Code) {
		t.Fatal("persisted invitation hash does not match issued code")
	}
}

func TestCreateInvitationRejectsInvalidEmail(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	body := bytes.NewBufferString(`{"email":"not-an-email"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/invitations", body)
	request.Header.Set("Authorization", "Bearer operator-secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid email, got %d: %s", response.Code, response.Body.String())
	}
}

func TestRequestMagicLinkSendsLinkForKnownHouseEmail(t *testing.T) {
	cfg := testConfig()
	cfg.Houses = []config.House{{ID: "house-1", Name: "Casa Silva", Email: "responsavel@example.com"}}
	cfg.Profiles[0].HouseID = "house-1"
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	mailer := &recordingMailer{}
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", mailer)

	body := bytes.NewBufferString(`{"email":"Responsavel@Example.com"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-links", body)
	request.Host = "teendns.lab.markun.com.br"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var result magicLinkResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "magic_link_sent" {
		t.Fatalf("expected magic_link_sent, got %q", result.Status)
	}
	if len(mailer.sent) != 1 || !strings.Contains(mailer.sent[0].html, "https://teendns.lab.markun.com.br/entrar?token=") {
		t.Fatalf("expected an email with a /entrar link, got %+v", mailer.sent)
	}
	if snapshot := server.Snapshot(); len(snapshot.Waitlist) != 0 {
		t.Fatalf("known email must not be waitlisted, got %+v", snapshot.Waitlist)
	}
}

func TestRequestMagicLinkWaitlistsUnknownEmailWithoutDuplicating(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	mailer := &recordingMailer{}
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", mailer)

	request := func() *httptest.ResponseRecorder {
		body := bytes.NewBufferString(`{"email":"nova@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-links", body)
		resp := httptest.NewRecorder()
		server.Handler().ServeHTTP(resp, req)
		return resp
	}

	first := request()
	if first.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", first.Code, first.Body.String())
	}
	var result magicLinkResponse
	if err := json.NewDecoder(first.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "waitlisted" {
		t.Fatalf("expected waitlisted, got %q", result.Status)
	}

	second := request()
	if second.Code != http.StatusOK {
		t.Fatalf("expected 200 on repeat request, got %d", second.Code)
	}

	snapshot := server.Snapshot()
	if len(snapshot.Waitlist) != 1 || snapshot.Waitlist[0].Email != "nova@example.com" {
		t.Fatalf("expected exactly one deduplicated waitlist entry, got %+v", snapshot.Waitlist)
	}
	if len(mailer.sent) != 2 {
		t.Fatalf("expected a confirmation email on each request, got %d", len(mailer.sent))
	}
}

func TestCreateSessionAuthorizesLikeHouseToken(t *testing.T) {
	cfg := testConfig()
	cfg.Houses = []config.House{{ID: "house-1", Name: "Casa Silva", Email: "responsavel@example.com"}}
	cfg.Profiles[0].HouseID = "house-1"
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	magicLinks := testMagicLinks()
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), magicLinks, testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	token, err := magicLinks.IssueLink("house-1")
	if err != nil {
		t.Fatal(err)
	}

	sessionBody := bytes.NewBufferString(`{"token":"` + token + `"}`)
	sessionRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sessions", sessionBody)
	sessionResponseRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(sessionResponseRecorder, sessionRequest)
	if sessionResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", sessionResponseRecorder.Code, sessionResponseRecorder.Body.String())
	}
	var session sessionResponse
	if err := json.NewDecoder(sessionResponseRecorder.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.SessionToken == "" {
		t.Fatal("expected a non-empty session token")
	}

	profilesRequest := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	profilesRequest.Header.Set("Authorization", "Bearer "+session.SessionToken)
	profilesResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(profilesResponse, profilesRequest)
	if profilesResponse.Code != http.StatusOK {
		t.Fatalf("expected session token to authorize like a house token, got %d: %s", profilesResponse.Code, profilesResponse.Body.String())
	}
	var result struct {
		Profiles []policy.Profile `json:"profiles"`
	}
	if err := json.NewDecoder(profilesResponse.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Profiles) != 1 || result.Profiles[0].HouseID != "house-1" {
		t.Fatalf("unexpected profiles for session-scoped request: %+v", result.Profiles)
	}

	replay := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sessions", bytes.NewBufferString(`{"token":"`+token+`"}`))
	replayResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(replayResponse, replay)
	if replayResponse.Code != http.StatusForbidden {
		t.Fatalf("expected a used link token to be rejected, got %d", replayResponse.Code)
	}
}

func TestCreateSessionRejectsUnknownToken(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sessions", bytes.NewBufferString(`{"token":"does-not-exist"}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for an unknown token, got %d", response.Code)
	}
}

func TestListHousesRequiresOperatorAndCountsProfiles(t *testing.T) {
	cfg := testConfig()
	cfg.Houses = []config.House{{ID: "house-1", Name: "Casa Silva", Email: "a@example.com"}}
	cfg.Profiles[0].HouseID = "house-1"
	cfg.Profiles = append(cfg.Profiles, policy.Profile{ID: "second", HouseID: "house-1", Hostname: "p-second.dns.teendns.test", DefaultAction: policy.ActionAllow, Version: 1})
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	unauthorized := httptest.NewRequest(http.MethodGet, "/api/v1/houses", nil)
	unauthorizedResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", unauthorizedResponse.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/houses", nil)
	request.Header.Set("Authorization", "Bearer operator-secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Houses []houseSummaryResponse `json:"houses"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Houses) != 1 || result.Houses[0].ProfileCount != 2 || result.Houses[0].Email != "a@example.com" {
		t.Fatalf("unexpected house summary: %+v", result.Houses)
	}
	if len(result.Houses[0].Emails) != 1 || result.Houses[0].Emails[0] != "a@example.com" {
		t.Fatalf("expected house summary to include admin emails, got %+v", result.Houses[0].Emails)
	}
}

func TestAdminGuardRequiresOperatorToken(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	missing := httptest.NewRecorder()
	server.Handler().ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/v1/admin/guard", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", missing.Code)
	}

	houseToken, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := server.update(func(cfg *config.Config) error {
		cfg.Houses = append(cfg.Houses, config.House{ID: "house-1", Name: "Casa", AdminTokenHash: tokenHash(houseToken)})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	houseRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/guard", nil)
	houseRequest.Header.Set("Authorization", "Bearer "+houseToken)
	houseResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(houseResponse, houseRequest)
	if houseResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected house token to be rejected by admin guard, got %d", houseResponse.Code)
	}

	operatorRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/guard", nil)
	operatorRequest.Header.Set("Authorization", "Bearer operator-secret")
	operatorResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(operatorResponse, operatorRequest)
	if operatorResponse.Code != http.StatusOK {
		t.Fatalf("expected operator token to pass, got %d: %s", operatorResponse.Code, operatorResponse.Body.String())
	}
}

func TestSetHouseEmailsReplacesLoginAddresses(t *testing.T) {
	cfg := testConfig()
	cfg.Houses = []config.House{{ID: "house-1", Name: "Casa Silva", Email: "pai@example.com"}}
	cfg.Profiles[0].HouseID = "house-1"
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	forbidden := httptest.NewRequest(http.MethodPut, "/api/v1/houses/house-1/emails", bytes.NewBufferString(`{"emails":["x@example.com"]}`))
	houseToken, _ := randomToken()
	if err := server.update(func(cfg *config.Config) error {
		cfg.Houses[0].AdminTokenHash = tokenHash(houseToken)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	forbidden.Header.Set("Authorization", "Bearer "+houseToken)
	forbiddenResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(forbiddenResponse, forbidden)
	if forbiddenResponse.Code != http.StatusForbidden {
		t.Fatalf("expected house token to be rejected with 403, got %d", forbiddenResponse.Code)
	}

	invalid := httptest.NewRequest(http.MethodPut, "/api/v1/houses/house-1/emails", bytes.NewBufferString(`{"emails":["nope"]}`))
	invalid.Header.Set("Authorization", "Bearer operator-secret")
	invalidResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an invalid email, got %d: %s", invalidResponse.Code, invalidResponse.Body.String())
	}

	body := bytes.NewBufferString(`{"emails":[" Pai@Example.com ", "mae@example.com", "pai@example.com", ""]}`)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/houses/house-1/emails", body)
	request.Header.Set("Authorization", "Bearer operator-secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var updated houseEmailsResponse
	if err := json.NewDecoder(response.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	want := []string{"pai@example.com", "mae@example.com"}
	if updated.Email != "pai@example.com" || !slices.Equal(updated.Emails, want) {
		t.Fatalf("unexpected updated emails: %+v", updated)
	}
	snapshot := server.Snapshot()
	if snap := snapshot.Houses[0]; snap.Email != "pai@example.com" || !slices.Equal(snap.Emails, want) {
		t.Fatalf("unexpected persisted house: %+v", snap)
	}

	missing := httptest.NewRequest(http.MethodPut, "/api/v1/houses/does-not-exist/emails", bytes.NewBufferString(`{"emails":["x@example.com"]}`))
	missing.Header.Set("Authorization", "Bearer operator-secret")
	missingResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(missingResponse, missing)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown house, got %d", missingResponse.Code)
	}
}

func TestHouseTimeZoneIsScopedAndActivatesForAllProfiles(t *testing.T) {
	cfg := testConfig()
	cfg.Houses = []config.House{{ID: "house-1", Name: "Casa", AdminTokenHash: tokenHash("house-secret")}}
	cfg.Profiles[0].HouseID = "house-1"
	cfg.Profiles = append(cfg.Profiles, policy.Profile{ID: "second", HouseID: "house-1", Label: "Outro", Hostname: "p-second.dns.teendns.test", DefaultAction: policy.ActionAllow, Version: 4, Rules: []policy.Rule{}})
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, err := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/home", bytes.NewBufferString(`{"label":"Casa","default_action":"allow","rules":[],"groups":[],"pauses":[],"time_zone":"America/Manaus"}`))
	request.Header.Set("Authorization", "Bearer house-secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	profile, ok := manager.Profile("p-home.dns.teendns.test")
	if !ok || profile.TimeZone != "America/Manaus" || profile.Location == nil || profile.Location.String() != "America/Manaus" {
		t.Fatalf("house time zone was not activated for profile: %+v", profile)
	}
	second, ok := manager.Profile("p-second.dns.teendns.test")
	if !ok || second.TimeZone != "America/Manaus" || second.Version != 5 {
		t.Fatalf("house time zone was not activated and versioned for its other profiles: %+v", second)
	}

	getZone := httptest.NewRequest(http.MethodGet, "/api/v1/houses/house-1/timezone", nil)
	getZone.Header.Set("Authorization", "Bearer house-secret")
	zoneResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(zoneResponse, getZone)
	var saved houseTimeZoneResponse
	if err := json.NewDecoder(zoneResponse.Body).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	if zoneResponse.Code != http.StatusOK || saved.TimeZone != "America/Manaus" {
		t.Fatalf("unexpected time zone response: %d %+v", zoneResponse.Code, saved)
	}
	persisted, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Houses[0].TimeZone != "America/Manaus" || persisted.Profiles[0].TimeZone != "America/Manaus" {
		t.Fatalf("house time zone was not persisted/applied: %+v", persisted)
	}

	invalid := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/home", bytes.NewBufferString(`{"label":"Casa","default_action":"allow","rules":[],"groups":[],"time_zone":"Mars/Olympus"}`))
	invalid.Header.Set("Authorization", "Bearer house-secret")
	invalidResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid time zone, got %d", invalidResponse.Code)
	}

	otherHouse := httptest.NewRequest(http.MethodGet, "/api/v1/houses/other-house/timezone", nil)
	otherHouse.Header.Set("Authorization", "Bearer house-secret")
	otherResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(otherResponse, otherHouse)
	if otherResponse.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an out-of-scope house, got %d", otherResponse.Code)
	}
}

func TestMergeGroupsPreservesSchedulesWhenClientOmitsField(t *testing.T) {
	existing := []policy.RuleGroup{{
		ID: "games", Name: "Jogos", Action: policy.ActionBlock, Domains: []string{"games.test"},
		Schedules: []policy.ScheduledAction{{TimeWindow: policy.TimeWindow{ID: "after-school", Label: "Depois da escola", Days: []int{1}, Start: "16:00", End: "18:00"}, Action: policy.ActionAllow}},
	}}
	merged, err := mergeGroups(existing, []policy.RuleGroup{{ID: "games", Name: "Jogos", Action: policy.ActionBlock, Domains: []string{"games.test"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 1 || len(merged[0].Schedules) != 1 || merged[0].Schedules[0].ID != "after-school" {
		t.Fatalf("expected omitted schedule to survive an older client update: %+v", merged)
	}
}

func TestRequestMagicLinkAcceptsAnyHouseAdminEmail(t *testing.T) {
	cfg := testConfig()
	cfg.Houses = []config.House{{ID: "house-1", Name: "Casa Silva", Email: "pai@example.com", Emails: []string{"pai@example.com", "mae@example.com"}}}
	cfg.Profiles[0].HouseID = "house-1"
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	mailer := &recordingMailer{}
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", mailer)

	for _, email := range []string{"pai@example.com", "mae@example.com", "MAE@example.com"} {
		body := bytes.NewBufferString(`{"email":"` + email + `"}`)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-links", body)
		request.Host = "teendns.lab.markun.com.br"
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200 for %q, got %d: %s", email, response.Code, response.Body.String())
		}
		var result magicLinkResponse
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.Status != "magic_link_sent" {
			t.Fatalf("expected magic_link_sent for %q, got %q", email, result.Status)
		}
	}
	if len(mailer.sent) != 3 {
		t.Fatalf("expected three magic links sent, got %d", len(mailer.sent))
	}
	if snapshot := server.Snapshot(); len(snapshot.Waitlist) != 0 {
		t.Fatalf("admin emails must not be waitlisted, got %+v", snapshot.Waitlist)
	}
}

type purgeCall struct {
	houseID    string
	profileIDs []string
}

type recordingDigestPurger struct {
	calls []purgeCall
}

func (p *recordingDigestPurger) Forget(houseID string, profileIDs []string) error {
	p.calls = append(p.calls, purgeCall{houseID: houseID, profileIDs: append([]string{}, profileIDs...)})
	return nil
}

func TestDeleteHouseRemovesProfilesAndPurgesDigest(t *testing.T) {
	cfg := testConfig()
	cfg.Houses = []config.House{
		{ID: "house-1", Name: "Casa Silva", Email: "a@example.com"},
		{ID: "house-2", Name: "Casa Souza", Email: "b@example.com"},
	}
	cfg.Profiles[0].HouseID = "house-1"
	cfg.Profiles = append(cfg.Profiles,
		policy.Profile{ID: "second", HouseID: "house-1", Hostname: "p-second.dns.teendns.test", DefaultAction: policy.ActionAllow, Version: 1},
		policy.Profile{ID: "third", HouseID: "house-2", Hostname: "p-third.dns.teendns.test", DefaultAction: policy.ActionAllow, Version: 1},
	)
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	purger := &recordingDigestPurger{}
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), purger, "dns.teendns.test", "operator-secret", testMailer())

	forbidden := httptest.NewRequest(http.MethodDelete, "/api/v1/houses/house-1", nil)
	forbiddenResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(forbiddenResponse, forbidden)
	if forbiddenResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", forbiddenResponse.Code)
	}

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/houses/house-1", nil)
	request.Header.Set("Authorization", "Bearer operator-secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	snapshot := server.Snapshot()
	if len(snapshot.Houses) != 1 || snapshot.Houses[0].ID != "house-2" {
		t.Fatalf("expected only house-2 to remain, got %+v", snapshot.Houses)
	}
	if len(snapshot.Profiles) != 1 || snapshot.Profiles[0].ID != "third" {
		t.Fatalf("expected house-1's profiles to be removed, got %+v", snapshot.Profiles)
	}
	if len(purger.calls) != 1 || purger.calls[0].houseID != "house-1" || len(purger.calls[0].profileIDs) != 2 {
		t.Fatalf("expected digest data purged for house-1's two profiles, got %+v", purger.calls)
	}

	missing := httptest.NewRequest(http.MethodDelete, "/api/v1/houses/does-not-exist", nil)
	missing.Header.Set("Authorization", "Bearer operator-secret")
	missingResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(missingResponse, missing)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown house, got %d", missingResponse.Code)
	}
}

func TestListWaitlistRequiresOperator(t *testing.T) {
	cfg := testConfig()
	cfg.Waitlist = []config.WaitlistEntry{{Email: "espera@example.com", CreatedAt: time.Now()}}
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	request := httptest.NewRequest(http.MethodGet, "/api/v1/waitlist", nil)
	request.Header.Set("Authorization", "Bearer operator-secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Waitlist []waitlistEntryResponse `json:"waitlist"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Waitlist) != 1 || result.Waitlist[0].Email != "espera@example.com" {
		t.Fatalf("unexpected waitlist: %+v", result.Waitlist)
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

func TestCatalogPackagesListsReadyMadeGroups(t *testing.T) {
	t.Setenv("TEENDNS_CATALOG_DIR", filepath.Join("..", "..", "catalog", "v1"))
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "secret", testMailer())

	request := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/packages", nil)
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Packages []catalogPackageResponse `json:"packages"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Packages) != 15 {
		t.Fatalf("expected 15 packages, got %d", len(result.Packages))
	}
	instagram := slices.IndexFunc(result.Packages, func(item catalogPackageResponse) bool { return item.ID == "service-instagram" })
	if instagram < 0 || result.Packages[instagram].DomainCount != 3 || result.Packages[instagram].SuggestedAction != policy.ActionObserve {
		t.Fatalf("unexpected Instagram package: %+v", result.Packages)
	}
}

func TestProfilePackageCanBeEnabledAndDisabledImmediately(t *testing.T) {
	t.Setenv("TEENDNS_CATALOG_DIR", filepath.Join("..", "..", "catalog", "v1"))
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "secret", testMailer())

	put := func(body string) policy.Profile {
		request := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/home/packages/service-instagram", bytes.NewBufferString(body))
		request.Header.Set("Authorization", "Bearer secret")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}
		var updated policy.Profile
		if err := json.NewDecoder(response.Body).Decode(&updated); err != nil {
			t.Fatal(err)
		}
		return updated
	}

	enabled := put(`{"enabled":true,"action":"block"}`)
	if groupAction(enabled.Groups, "service-instagram") != policy.ActionBlock {
		t.Fatalf("Instagram package was not enabled: %+v", enabled.Groups)
	}
	active, ok := manager.Profile("p-home.dns.teendns.test")
	if !ok {
		t.Fatal("updated profile is not active")
	}
	decision, err := policy.Decide(active, "cdninstagram.com")
	if err != nil || decision.Action != policy.ActionBlock {
		t.Fatalf("package was not activated immediately: %+v, %v", decision, err)
	}

	disabled := put(`{"enabled":false,"action":"block"}`)
	if groupAction(disabled.Groups, "service-instagram") != "" {
		t.Fatalf("Instagram package was not removed: %+v", disabled.Groups)
	}
	active, _ = manager.Profile("p-home.dns.teendns.test")
	decision, err = policy.Decide(active, "cdninstagram.com")
	if err != nil || decision.Action != policy.ActionAllow {
		t.Fatalf("disabled package kept affecting decisions: %+v, %v", decision, err)
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
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "secret", testMailer())
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

func TestMergeGroupsKeepsExistingEmptyPlaceholder(t *testing.T) {
	existing := []policy.RuleGroup{{
		ID: "adult", Name: "Conteúdo adulto", Action: policy.ActionBlock, Domains: []string{},
	}}
	groups, err := mergeGroups(existing, existing)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Domains == nil || len(groups[0].Domains) != 0 {
		t.Fatalf("unexpected empty placeholder: %+v", groups)
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
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), pairings, testMagicLinks(), testDigestPurger(), "dns.teendns.test", "secret", testMailer())

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

// testMailer returns a Sender in log-only mode: no network call, no
// credential required, safe for every test that does not assert on the
// email itself.
func testMailer() mail.Sender {
	return mail.NewResendClient("", "")
}

func testMagicLinks() *magiclink.Manager {
	return magiclink.NewManager(time.Minute, time.Hour)
}

// noopDigestPurger satisfies DigestPurger for tests that never delete a
// house and so never need real purge behavior.
type noopDigestPurger struct{}

func (noopDigestPurger) Forget(string, []string) error { return nil }

func testDigestPurger() DigestPurger {
	return noopDigestPurger{}
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

func TestSetupDownloadsConfiguredFiles(t *testing.T) {
	t.Setenv("TEENDNS_DOH_BASE_URL", "https://teendns.lab.markun.com.br/dns-query/")
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "secret", testMailer())
	server.resolverIP = "203.0.113.10"

	get := func(kind string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/home/setup/"+kind, nil)
		request.Header.Set("Authorization", "Bearer secret")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		return response
	}

	rec := get("windows.bat")
	if rec.Code != http.StatusOK {
		t.Fatalf("windows.bat: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("unexpected content type %q", contentType)
	}
	if disposition := rec.Header().Get("Content-Disposition"); disposition == "" || !strings.Contains(disposition, "configurar-teendns.bat") {
		t.Errorf("unexpected content disposition %q", disposition)
	}
	for _, want := range []string{`set "TEENDNS_HOST=p-home.dns.teendns.test"`, "dothost=%TEENDNS_HOST%:%TEENDNS_PORT%", "203.0.113.10"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("windows.bat missing %q", want)
		}
	}

	rec = get("windows-remove.bat")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "remover-teendns.bat") {
		t.Fatalf("windows-remove.bat: %d disposition %q", rec.Code, rec.Header().Get("Content-Disposition"))
	}

	rec = get("apple.mobileconfig")
	if rec.Code != http.StatusOK {
		t.Fatalf("apple.mobileconfig: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "application/x-apple-aspen-config; charset=utf-8" {
		t.Errorf("unexpected content type %q", contentType)
	}
	for _, want := range []string{"com.apple.dnsSettings.managed", "<string>TLS</string>", "p-home.dns.teendns.test", "203.0.113.10"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("apple.mobileconfig missing %q", want)
		}
	}

	rec = get("info")
	if rec.Code != http.StatusOK {
		t.Fatalf("info: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"hostname":"p-home.dns.teendns.test"`, `"ip":"203.0.113.10"`, `"port":"853"`, `"test_domain":"example.com"`, `"doh_url":"https://teendns.lab.markun.com.br/dns-query/p-home"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("info missing %q", want)
		}
	}
}

func TestNormalizeDoHBaseURLRequiresHTTPSAndExpectedPath(t *testing.T) {
	base, err := normalizeDoHBaseURL(" https://teendns.lab.markun.com.br/dns-query/ ")
	if err != nil || base != "https://teendns.lab.markun.com.br/dns-query" {
		t.Fatalf("unexpected normalized DoH base URL %q, %v", base, err)
	}
	for _, invalid := range []string{
		"http://teendns.lab.markun.com.br/dns-query",
		"https://user:password@teendns.lab.markun.com.br/dns-query",
		"https://teendns.lab.markun.com.br/other",
		"https://teendns.lab.markun.com.br/dns-query?profile=secret",
	} {
		if _, err := normalizeDoHBaseURL(invalid); err == nil {
			t.Errorf("expected invalid DoH base URL %q to be rejected", invalid)
		}
	}
}

func TestSetupAppleOmitsServerAddressesWithoutPublicIP(t *testing.T) {
	cfg := testConfig()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "secret", testMailer())
	server.resolverIP = ""

	request := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/home/setup/apple.mobileconfig", nil)
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "ServerAddresses") {
		t.Errorf("mobileconfig without public IP must omit ServerAddresses")
	}
}

func TestSetupRefusesForeignHouseAndDisabledProfile(t *testing.T) {
	cfg := config.Config{
		Listen:      ":853",
		Upstream:    "unbound:5353",
		Certificate: "cert.pem",
		PrivateKey:  "key.pem",
		MaxTTL:      300,
		Houses: []config.House{
			{ID: "house-a", Name: "Casa A", AdminTokenHash: tokenHash("house-key-a")},
		},
		Profiles: []policy.Profile{
			{ID: "a", HouseID: "house-a", Hostname: "p-a.dns.teendns.test", DefaultAction: policy.ActionAllow, Version: 1},
			{ID: "b", HouseID: "house-b", Hostname: "p-b.dns.teendns.test", DefaultAction: policy.ActionAllow, Version: 1},
			{ID: "gone", Disabled: true, Hostname: "p-gone.dns.teendns.test", DefaultAction: policy.ActionAllow, Version: 1},
		},
	}
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), testPairing(), testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	get := func(token, endpoint string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, endpoint, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		return response
	}

	if rec := get("house-key-a", "/api/v1/profiles/b/setup/windows.bat"); rec.Code != http.StatusNotFound {
		t.Errorf("house A must not download house B query config, got %d", rec.Code)
	}
	if rec := get("operator-secret", "/api/v1/profiles/gone/setup/apple.mobileconfig"); rec.Code != http.StatusNotFound {
		t.Errorf("disabled profile must not download config, got %d", rec.Code)
	}
	if rec := get("operator-secret", "/api/v1/profiles/b/setup/unknown"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown setup kind must be 404, got %d", rec.Code)
	}
}

func TestPairingOutcomeScopedToObservingHouse(t *testing.T) {
	cfg := config.Config{
		Listen:      ":853",
		Upstream:    "unbound:5353",
		Certificate: "cert.pem",
		PrivateKey:  "key.pem",
		MaxTTL:      300,
		Houses: []config.House{
			{ID: "house-a", Name: "Casa A", AdminTokenHash: tokenHash("house-key-a")},
		},
		Profiles: []policy.Profile{
			{ID: "a", HouseID: "house-a", Hostname: "p-a.dns.teendns.test", DefaultAction: policy.ActionAllow, Version: 1},
			{ID: "b", HouseID: "house-b", Hostname: "p-b.dns.teendns.test", DefaultAction: policy.ActionAllow, Version: 1},
		},
	}
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := config.WriteAtomic(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, _ := policy.NewManager(cfg.Profiles)
	pairings := testPairing()
	server, _ := NewServer(path, cfg, manager, gateway.NewEventBuffer(), pairings, testMagicLinks(), testDigestPurger(), "dns.teendns.test", "operator-secret", testMailer())

	challenge, err := pairings.Create()
	if err != nil {
		t.Fatal(err)
	}

	check := func(token, challengeID string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/pairing/challenges/"+challengeID+"/outcome", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		return response
	}

	if rec := check("house-key-a", challenge.ID); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"observed":false`) {
		t.Fatalf("pending challenge must report unobserved, got %d %s", rec.Code, rec.Body.String())
	}
	if !pairings.Observe("b", challenge.DNSName) {
		t.Fatal("observe challenge failed")
	}
	if rec := check("house-key-a", challenge.ID); rec.Code != http.StatusNotFound {
		t.Fatalf("house A must not see house B outcome, got %d", rec.Code)
	}
	second, _ := pairings.Create()
	if !pairings.Observe("a", second.DNSName) {
		t.Fatal("observe second challenge failed")
	}
	if rec := check("house-key-a", second.ID); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"observed":true`) || !strings.Contains(rec.Body.String(), `"profile_id":"a"`) {
		t.Fatalf("house A must see its own profile outcome, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := check("operator-secret", challenge.ID); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"profile_id":"b"`) {
		t.Fatalf("operator must see any observed profile, got %d %s", rec.Code, rec.Body.String())
	}
}
