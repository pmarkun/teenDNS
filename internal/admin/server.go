package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/gateway"
	"github.com/pmarkun/teendns/internal/magiclink"
	"github.com/pmarkun/teendns/internal/mail"
	"github.com/pmarkun/teendns/internal/pairing"
	"github.com/pmarkun/teendns/internal/policy"
	"github.com/pmarkun/teendns/internal/setup"
)

// DigestPurger removes a deleted house's accumulated digest data. Satisfied
// by *digest.Store; kept as a narrow interface so admin does not need to
// import the digest package for anything else.
type DigestPurger interface {
	Forget(houseID string, profileIDs []string) error
}

type Server struct {
	mu             sync.RWMutex
	configPath     string
	config         config.Config
	profiles       *policy.Manager
	events         *gateway.EventBuffer
	pairings       *pairing.Manager
	magicLinks     *magiclink.Manager
	digest         DigestPurger
	hostnameSuffix string
	resolverIP     string
	dnsPort        string
	testDomain     string
	token          string
	catalogDir     string
	mailer         mail.Sender
}

type accessScope struct {
	operator bool
	houseID  string
}

type accessContextKey struct{}

type profileRequest struct {
	Label         string               `json:"label"`
	DefaultAction policy.Action        `json:"default_action"`
	Rules         []policy.Rule        `json:"rules"`
	Groups        []policy.RuleGroup   `json:"groups"`
	Pauses        *[]policy.TimeWindow `json:"pauses"`
	TimeZone      *string              `json:"time_zone"`
}

type houseTimeZoneResponse struct {
	TimeZone string `json:"time_zone"`
}

type registrationRequest struct {
	InvitationCode string `json:"invitation_code"`
	HouseName      string `json:"house_name"`
	ProfileName    string `json:"profile_name"`
	Preset         string `json:"preset"`
}

type registrationResponse struct {
	House      houseResponse  `json:"house"`
	AdminToken string         `json:"admin_token"`
	Profile    policy.Profile `json:"profile"`
}

type houseResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type invitationRequest struct {
	Email string `json:"email"`
}

type invitationResponse struct {
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
	Code      string    `json:"code"`
	Link      string    `json:"link"`
	EmailSent bool      `json:"email_sent"`
}

const invitationValidity = 7 * 24 * time.Hour

type magicLinkRequest struct {
	Email string `json:"email"`
}

type magicLinkResponse struct {
	Status string `json:"status"`
}

type sessionRequest struct {
	Token string `json:"token"`
}

type sessionResponse struct {
	SessionToken string `json:"session_token"`
}

type houseSummaryResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Email        string   `json:"email,omitempty"`
	Emails       []string `json:"emails"`
	ProfileCount int      `json:"profile_count"`
}

type houseEmailsRequest struct {
	Emails []string `json:"emails"`
}

type houseEmailsResponse struct {
	ID     string   `json:"id"`
	Email  string   `json:"email,omitempty"`
	Emails []string `json:"emails"`
}

type waitlistEntryResponse struct {
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type catalogPackageResponse struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Category        string        `json:"category"`
	Reason          string        `json:"reason"`
	DomainCount     int           `json:"domain_count"`
	SuggestedAction policy.Action `json:"suggested_action"`
}

type packageRequest struct {
	Enabled bool          `json:"enabled"`
	Action  policy.Action `json:"action"`
}

type youthRule struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type youthProfile struct {
	Label string      `json:"label"`
	Rules []youthRule `json:"rules"`
}

// pairingOutcomeResponse is the scoped counterpart of pairing.Status: it tells
// the panel whether a device already resolved the challenge and, if so, via
// which of the house's profiles — without ever exposing a session token.
type pairingOutcomeResponse struct {
	Observed  bool   `json:"observed"`
	ProfileID string `json:"profile_id,omitempty"`
}

// setupInfoResponse carries the neutral values for the manual configuration
// section: hostname, public resolver IP (may be empty in the lab), DoT port
// and the domain used by scripts for their resolution test.
type setupInfoResponse struct {
	Hostname   string `json:"hostname"`
	IP         string `json:"ip"`
	Port       string `json:"port"`
	TestDomain string `json:"test_domain"`
}

const (
	defaultDNSPort       = "853"
	defaultDNSTestDomain = "example.com"
	setupBatchFilename   = "configurar-teendns.bat"
	setupRemoveFilename  = "remover-teendns.bat"
	setupProfileFilename = "teendns.mobileconfig"
	setupWindowsMime     = "text/plain; charset=utf-8"
	setupAppleMime       = "application/x-apple-aspen-config; charset=utf-8"
	setupInfoMime        = "application/json; charset=utf-8"
)

func NewServer(configPath string, cfg config.Config, profiles *policy.Manager, events *gateway.EventBuffer, pairings *pairing.Manager, magicLinks *magiclink.Manager, digest DigestPurger, hostnameSuffix, token string, mailer mail.Sender) (*Server, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("admin token is required")
	}
	if pairings == nil {
		return nil, errors.New("pairing manager is required")
	}
	if magicLinks == nil {
		return nil, errors.New("magic link manager is required")
	}
	if digest == nil {
		return nil, errors.New("digest purger is required")
	}
	if mailer == nil {
		return nil, errors.New("mailer is required")
	}
	if err := config.ApplyHouseTimeZones(&cfg); err != nil {
		return nil, err
	}
	if err := profiles.Replace(cfg.Profiles); err != nil {
		return nil, err
	}
	if hostnameSuffix == "" {
		hostnameSuffix = "dns.teendns.test"
	}
	normalizedSuffix := strings.TrimSuffix(strings.ToLower(hostnameSuffix), ".")
	resolverIP := strings.TrimSpace(os.Getenv("TEENDNS_DNS_PUBLIC_IP"))
	if resolverIP == "" {
		resolverIP = setup.ResolverIP(normalizedSuffix)
	}
	return &Server{
		configPath:     configPath,
		config:         cloneConfig(cfg),
		profiles:       profiles,
		events:         events,
		pairings:       pairings,
		magicLinks:     magicLinks,
		digest:         digest,
		hostnameSuffix: normalizedSuffix,
		resolverIP:     resolverIP,
		dnsPort:        environmentOrDefault("TEENDNS_DNS_PORT", defaultDNSPort),
		testDomain:     environmentOrDefault("TEENDNS_DNS_TEST_DOMAIN", defaultDNSTestDomain),
		token:          token,
		catalogDir:     environmentOrDefault("TEENDNS_CATALOG_DIR", "catalog/v1"),
		mailer:         mailer,
	}, nil
}

// Reload keeps the control plane in sync when an operator uses the legacy
// SIGHUP configuration path.
func (s *Server) Reload(cfg config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cloneConfig(cfg)
}

// Snapshot returns a copy of the current configuration for read-only
// consumers outside the admin API, such as the digest scheduler.
func (s *Server) Snapshot() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneConfig(s.config)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/admin/guard", s.adminGuard)
	mux.HandleFunc("POST /api/v1/pairing/challenges", s.createPairingChallenge)
	mux.HandleFunc("GET /api/v1/pairing/challenges/{id}", s.pairingChallengeStatus)
	mux.HandleFunc("GET /api/v1/pairing/challenges/{id}/outcome", s.authorize(s.pairingChallengeOutcome))
	mux.HandleFunc("GET /api/v1/youth/profile", s.youthProfile)
	mux.HandleFunc("POST /api/v1/houses", s.registerHouse)
	mux.HandleFunc("GET /api/v1/houses", s.authorize(s.listHouses))
	mux.HandleFunc("DELETE /api/v1/houses/{id}", s.authorize(s.deleteHouse))
	mux.HandleFunc("GET /api/v1/houses/{id}/timezone", s.authorize(s.houseTimeZone))
	mux.HandleFunc("PUT /api/v1/houses/{id}/emails", s.authorize(s.setHouseEmails))
	mux.HandleFunc("POST /api/v1/invitations", s.authorize(s.createInvitation))
	mux.HandleFunc("GET /api/v1/waitlist", s.authorize(s.listWaitlist))
	mux.HandleFunc("POST /api/v1/auth/magic-links", s.requestMagicLink)
	mux.HandleFunc("POST /api/v1/auth/sessions", s.createSession)
	mux.HandleFunc("GET /api/v1/catalog/packages", s.authorize(s.catalogPackages))
	mux.HandleFunc("/api/v1/profiles", s.authorize(s.profilesCollection))
	mux.HandleFunc("/api/v1/profiles/", s.authorize(s.profileResource))
	return withCORS(mux)
}

func (s *Server) catalogPackages(writer http.ResponseWriter, _ *http.Request) {
	groups, err := availablePackageGroups(s.catalogDir)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	packages := make([]catalogPackageResponse, 0, len(groups))
	for _, group := range groups {
		packages = append(packages, catalogPackageResponse{
			ID: group.ID, Name: group.Name, Category: group.Category, Reason: group.Reason,
			DomainCount: len(group.Domains), SuggestedAction: group.Action,
		})
	}
	writeJSON(writer, http.StatusOK, map[string]any{"packages": packages})
}

func (s *Server) registerHouse(writer http.ResponseWriter, request *http.Request) {
	var input registrationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	input.HouseName = strings.TrimSpace(input.HouseName)
	input.ProfileName = strings.TrimSpace(input.ProfileName)
	if input.HouseName == "" || input.ProfileName == "" {
		writeError(writer, http.StatusBadRequest, errors.New("nome da casa e do primeiro perfil são obrigatórios"))
		return
	}
	if len([]rune(input.HouseName)) > 80 || len([]rune(input.ProfileName)) > 80 {
		writeError(writer, http.StatusBadRequest, errors.New("use nomes com até 80 caracteres"))
		return
	}
	invitationKey := tokenHash(strings.TrimSpace(input.InvitationCode))
	s.mu.RLock()
	invitation, invitationValid := findValidInvitation(s.config.Invitations, invitationKey, time.Now())
	s.mu.RUnlock()
	if !invitationValid {
		writeError(writer, http.StatusForbidden, errors.New("convite inválido, expirado ou já usado"))
		return
	}
	houseToken, err := randomToken()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	houseIDToken, err := randomToken()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	profileToken, err := randomToken()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	house := config.House{ID: houseIDToken[:12], Name: input.HouseName, AdminTokenHash: tokenHash(houseToken), TimeZone: policy.DefaultTimeZone, Email: invitation.Email}
	if invitation.Email != "" {
		house.Emails = []string{invitation.Email}
	}
	groups, err := presetGroups(input.Preset, s.catalogDir)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	profile := policy.Profile{
		ID:            profileToken[:12],
		HouseID:       house.ID,
		Label:         input.ProfileName,
		Hostname:      "p-" + profileToken + "." + s.hostnameSuffix,
		DefaultAction: policy.ActionAllow,
		Version:       1,
		Rules:         []policy.Rule{},
		Groups:        groups,
	}
	err = s.update(func(cfg *config.Config) error {
		index, ok := findValidInvitationIndex(cfg.Invitations, invitationKey, time.Now())
		if !ok {
			return errors.New("convite inválido, expirado ou já usado")
		}
		now := time.Now()
		cfg.Invitations[index].UsedAt = &now
		cfg.Invitations[index].Email = ""
		cfg.Houses = append(cfg.Houses, house)
		cfg.Profiles = append(cfg.Profiles, profile)
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusForbidden, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusCreated, registrationResponse{
		House: houseResponse{ID: house.ID, Name: house.Name}, AdminToken: houseToken, Profile: profile,
	})
}

// createInvitation lets the operator generate a single-use, expiring
// invitation for a specific family and email it to them. Only the global
// operator token may call this — a house's own admin token cannot invite
// other houses.
func (s *Server) createInvitation(writer http.ResponseWriter, request *http.Request) {
	if !requestScope(request).operator {
		writeError(writer, http.StatusForbidden, errors.New("somente o operador pode gerar convites"))
		return
	}
	var input invitationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	input.Email = strings.TrimSpace(input.Email)
	if !looksLikeEmail(input.Email) {
		writeError(writer, http.StatusBadRequest, errors.New("e-mail inválido"))
		return
	}
	code, err := randomToken()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	now := time.Now()
	invitation := config.Invitation{
		CodeHash:  tokenHash(code),
		Email:     input.Email,
		CreatedAt: now,
		ExpiresAt: now.Add(invitationValidity),
	}
	if err := s.update(func(cfg *config.Config) error {
		cfg.Invitations = append(cfg.Invitations, invitation)
		return nil
	}); err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}

	link := s.publicLink(request, "/comecar", "convite", code)
	sendErr := s.mailer.Send(input.Email, "Convite para o teenDNS", invitationEmailBody(link))
	if sendErr != nil {
		log.Printf("invitation: failed to email %s: %v", input.Email, sendErr)
	}

	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusCreated, invitationResponse{
		Email:     input.Email,
		ExpiresAt: invitation.ExpiresAt,
		Code:      code,
		Link:      link,
		EmailSent: sendErr == nil,
	})
}

// publicLink builds an absolute link to a page of the public web app, e.g.
// /comecar?convite=<code> or /entrar?token=<token>. TEENDNS_PUBLIC_URL
// overrides the default of deriving the origin from the request's Host
// header (which nginx forwards unchanged from the original client request).
func (s *Server) publicLink(request *http.Request, path, param, value string) string {
	base := strings.TrimSuffix(environmentOrDefault("TEENDNS_PUBLIC_URL", ""), "/")
	if base == "" {
		base = "https://" + request.Host
	}
	return base + path + "?" + param + "=" + url.QueryEscape(value)
}

func invitationEmailBody(link string) string {
	return fmt.Sprintf(
		"<p>Você foi convidado a criar uma casa no teenDNS.</p><p><a href=\"%[1]s\">%[1]s</a></p><p>Este link é de uso único e expira em 7 dias.</p>",
		html.EscapeString(link),
	)
}

func magicLinkEmailBody(link string) string {
	return fmt.Sprintf(
		"<p>Use o link abaixo para entrar no painel do teenDNS.</p><p><a href=\"%[1]s\">%[1]s</a></p><p>Este link é de uso único e expira em 15 minutos.</p>",
		html.EscapeString(link),
	)
}

func waitlistEmailBody() string {
	return "<p>Recebemos seu pedido de acesso ao teenDNS. Ainda não encontramos uma casa com este e-mail, então você entrou na nossa lista de espera — avisamos assim que houver um convite disponível.</p>"
}

func looksLikeEmail(value string) bool {
	at := strings.IndexByte(value, '@')
	if at <= 0 || at == len(value)-1 {
		return false
	}
	domain := value[at+1:]
	return strings.Contains(domain, ".") && !strings.ContainsAny(value, " \t\n")
}

// houseHasEmail reports whether email is one of the addresses the house
// registered for magic-link login, including the primary one.
func houseHasEmail(house config.House, email string) bool {
	if strings.EqualFold(house.Email, email) {
		return true
	}
	for _, candidate := range house.Emails {
		if strings.EqualFold(candidate, email) {
			return true
		}
	}
	return false
}

// houseEmails returns the effective list of admin addresses for a house,
// falling back to the primary email so hand-built configurations behave the
// same as loaded ones (where Load migrates Email into Emails).
func houseEmails(house config.House) []string {
	if len(house.Emails) == 0 {
		if house.Email != "" {
			return []string{house.Email}
		}
		return []string{}
	}
	return house.Emails
}

// normalizeEmails trims, lowercases and deduplicates a list of addresses,
// rejecting anything that does not look like an email.
func normalizeEmails(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, raw := range values {
		email := strings.ToLower(strings.TrimSpace(raw))
		if email == "" {
			continue
		}
		if !looksLikeEmail(email) {
			return nil, fmt.Errorf("e-mail inválido: %q", raw)
		}
		if _, exists := seen[email]; exists {
			continue
		}
		seen[email] = struct{}{}
		result = append(result, email)
	}
	return result, nil
}

// requestMagicLink is the public entry point of the email-first login flow:
// a house with this email gets a one-time login link; anyone else gets
// added to the waitlist for an operator to invite later.
func (s *Server) requestMagicLink(writer http.ResponseWriter, request *http.Request) {
	var input magicLinkRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !looksLikeEmail(email) {
		writeError(writer, http.StatusBadRequest, errors.New("e-mail inválido"))
		return
	}

	s.mu.RLock()
	var houseID string
	for _, house := range s.config.Houses {
		if houseHasEmail(house, email) {
			houseID = house.ID
			break
		}
	}
	s.mu.RUnlock()

	writer.Header().Set("Cache-Control", "no-store")
	if houseID == "" {
		if err := s.update(func(cfg *config.Config) error {
			for _, entry := range cfg.Waitlist {
				if strings.EqualFold(entry.Email, email) {
					return nil
				}
			}
			cfg.Waitlist = append(cfg.Waitlist, config.WaitlistEntry{Email: email, CreatedAt: time.Now()})
			return nil
		}); err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}
		if err := s.mailer.Send(email, "Lista de espera do teenDNS", waitlistEmailBody()); err != nil {
			log.Printf("waitlist: failed to email %s: %v", email, err)
		}
		writeJSON(writer, http.StatusOK, magicLinkResponse{Status: "waitlisted"})
		return
	}

	token, err := s.magicLinks.IssueLink(houseID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	link := s.publicLink(request, "/entrar", "token", token)
	if err := s.mailer.Send(email, "Seu link de acesso ao teenDNS", magicLinkEmailBody(link)); err != nil {
		log.Printf("magic link: failed to email %s: %v", email, err)
	}
	writeJSON(writer, http.StatusOK, magicLinkResponse{Status: "magic_link_sent"})
}

// createSession exchanges a one-time magic link token for a session token
// that authorize() accepts exactly like a house's permanent admin token,
// without ever touching that permanent token.
func (s *Server) createSession(writer http.ResponseWriter, request *http.Request) {
	var input sessionRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	_, sessionToken, ok := s.magicLinks.Redeem(strings.TrimSpace(input.Token))
	if !ok {
		writeError(writer, http.StatusForbidden, errors.New("link inválido ou expirado"))
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusCreated, sessionResponse{SessionToken: sessionToken})
}

// adminGuard is the server-side gate for the operator console: it returns
// 200 only when the Authorization header carries the operator token, so the
// web app can refuse to render any admin content without a valid credential.
func (s *Server) adminGuard(writer http.ResponseWriter, request *http.Request) {
	provided := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	if len(provided) == len(s.token) && subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) == 1 {
		writeJSON(writer, http.StatusOK, map[string]bool{"operator": true})
		return
	}
	writeError(writer, http.StatusUnauthorized, errors.New("chave de operador inválida"))
}

func (s *Server) houseTimeZone(writer http.ResponseWriter, request *http.Request) {
	houseID := request.PathValue("id")
	scope := requestScope(request)
	s.mu.RLock()
	index := slices.IndexFunc(s.config.Houses, func(house config.House) bool { return house.ID == houseID })
	if index < 0 || (!scope.operator && scope.houseID != houseID) {
		s.mu.RUnlock()
		writeError(writer, http.StatusNotFound, errors.New("casa não encontrada"))
		return
	}
	zone := s.config.Houses[index].TimeZone
	if zone == "" {
		zone = policy.DefaultTimeZone
	}
	s.mu.RUnlock()
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", "GET")
		writeError(writer, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	writeJSON(writer, http.StatusOK, houseTimeZoneResponse{TimeZone: zone})
}

// setHouseEmails replaces the list of addresses allowed to log into a house's
// panel via magic link. Only the global operator may change it; the first
// address stays as the house's primary email, used for the weekly digest.
func (s *Server) setHouseEmails(writer http.ResponseWriter, request *http.Request) {
	if !requestScope(request).operator {
		writeError(writer, http.StatusForbidden, errors.New("somente o operador pode editar os e-mails da casa"))
		return
	}
	var input houseEmailsRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	emails, err := normalizeEmails(input.Emails)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	houseID := request.PathValue("id")
	var updated config.House
	err = s.update(func(cfg *config.Config) error {
		index := slices.IndexFunc(cfg.Houses, func(house config.House) bool { return house.ID == houseID })
		if index < 0 {
			return errors.New("casa não encontrada")
		}
		cfg.Houses[index].Emails = emails
		if len(emails) > 0 {
			cfg.Houses[index].Email = emails[0]
		} else {
			cfg.Houses[index].Email = ""
		}
		updated = cfg.Houses[index]
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, err)
		return
	}
	writeJSON(writer, http.StatusOK, houseEmailsResponse{ID: updated.ID, Email: updated.Email, Emails: append([]string{}, updated.Emails...)})
}

func (s *Server) listHouses(writer http.ResponseWriter, request *http.Request) {
	if !requestScope(request).operator {
		writeError(writer, http.StatusForbidden, errors.New("somente o operador pode listar casas"))
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	houses := make([]houseSummaryResponse, 0, len(s.config.Houses))
	for _, house := range s.config.Houses {
		count := 0
		for _, profile := range s.config.Profiles {
			if profile.HouseID == house.ID {
				count++
			}
		}
		houses = append(houses, houseSummaryResponse{ID: house.ID, Name: house.Name, Email: house.Email, Emails: append([]string{}, houseEmails(house)...), ProfileCount: count})
	}
	writeJSON(writer, http.StatusOK, map[string]any{"houses": houses})
}

// deleteHouse removes a house and every one of its profiles, then purges
// any digest data accumulated for those profiles. It is irreversible: the
// family's DNS profiles stop resolving immediately.
func (s *Server) deleteHouse(writer http.ResponseWriter, request *http.Request) {
	if !requestScope(request).operator {
		writeError(writer, http.StatusForbidden, errors.New("somente o operador pode apagar casas"))
		return
	}
	houseID := request.PathValue("id")
	var removedProfileIDs []string
	err := s.update(func(cfg *config.Config) error {
		index := slices.IndexFunc(cfg.Houses, func(house config.House) bool { return house.ID == houseID })
		if index < 0 {
			return errors.New("casa não encontrada")
		}
		cfg.Houses = slices.Delete(cfg.Houses, index, index+1)
		remaining := make([]policy.Profile, 0, len(cfg.Profiles))
		for _, profile := range cfg.Profiles {
			if profile.HouseID == houseID {
				removedProfileIDs = append(removedProfileIDs, profile.ID)
				continue
			}
			remaining = append(remaining, profile)
		}
		cfg.Profiles = remaining
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusNotFound, err)
		return
	}
	if err := s.digest.Forget(houseID, removedProfileIDs); err != nil {
		log.Printf("delete house %s: failed to purge digest data: %v", houseID, err)
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) listWaitlist(writer http.ResponseWriter, request *http.Request) {
	if !requestScope(request).operator {
		writeError(writer, http.StatusForbidden, errors.New("somente o operador pode ver a lista de espera"))
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]waitlistEntryResponse, 0, len(s.config.Waitlist))
	for _, entry := range s.config.Waitlist {
		entries = append(entries, waitlistEntryResponse{Email: entry.Email, CreatedAt: entry.CreatedAt})
	}
	writeJSON(writer, http.StatusOK, map[string]any{"waitlist": entries})
}

func (s *Server) createPairingChallenge(writer http.ResponseWriter, _ *http.Request) {
	challenge, err := s.pairings.Create()
	if err != nil {
		writeError(writer, http.StatusServiceUnavailable, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusCreated, challenge)
}

func (s *Server) pairingChallengeStatus(writer http.ResponseWriter, request *http.Request) {
	status, ok := s.pairings.Status(request.PathValue("id"))
	if !ok {
		writeError(writer, http.StatusNotFound, errors.New("pairing challenge not found or expired"))
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusOK, status)
}

// pairingChallengeOutcome reports, for the responsible side of the flow, which
// profile (if any) observed a challenge's DNS query. It is scoped to the
// authenticated house so a house cannot probe challenges it does not own.
func (s *Server) pairingChallengeOutcome(writer http.ResponseWriter, request *http.Request) {
	profileID, ok := s.pairings.Outcome(request.PathValue("id"))
	if !ok {
		writeError(writer, http.StatusNotFound, errors.New("pairing challenge not found or expired"))
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	if profileID == "" {
		writeJSON(writer, http.StatusOK, pairingOutcomeResponse{Observed: false})
		return
	}
	profile, ok := s.findProfile(profileID)
	if !ok || !scopeAllows(requestScope(request), profile) {
		writeError(writer, http.StatusNotFound, errors.New("perfil não encontrado"))
		return
	}
	writeJSON(writer, http.StatusOK, pairingOutcomeResponse{Observed: true, ProfileID: profileID})
}

func (s *Server) youthProfile(writer http.ResponseWriter, request *http.Request) {
	sessionToken := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	profileID, ok := s.pairings.Profile(sessionToken)
	if !ok {
		writeError(writer, http.StatusUnauthorized, errors.New("invalid or expired pairing session"))
		return
	}
	profile, ok := s.findProfile(profileID)
	if !ok || profile.Disabled {
		writeError(writer, http.StatusNotFound, errors.New("profile not found"))
		return
	}
	result := youthProfile{Label: profile.Label, Rules: []youthRule{}}
	for _, group := range profile.Groups {
		if group.Action != policy.ActionBlock {
			continue
		}
		result.Rules = append(result.Rules, youthRule{Name: group.Name, Reason: group.Reason})
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusOK, result)
}

func (s *Server) profilesCollection(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		s.mu.RLock()
		profiles := filterProfiles(s.config.Profiles, requestScope(request))
		s.mu.RUnlock()
		writeJSON(writer, http.StatusOK, map[string]any{"profiles": profiles})
	case http.MethodPost:
		var input profileRequest
		if err := decodeJSON(request, &input); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		input.Label = strings.TrimSpace(input.Label)
		if input.Label == "" {
			writeError(writer, http.StatusBadRequest, errors.New("label is required"))
			return
		}
		token, err := randomToken()
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}
		profile := policy.Profile{
			ID:            token[:12],
			HouseID:       requestScope(request).houseID,
			Label:         input.Label,
			Hostname:      "p-" + token + "." + s.hostnameSuffix,
			DefaultAction: policy.ActionAllow,
			Version:       1,
			Rules:         []policy.Rule{},
			Groups:        []policy.RuleGroup{},
		}
		if err := s.update(func(cfg *config.Config) error {
			cfg.Profiles = append(cfg.Profiles, profile)
			return nil
		}); err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}
		writeJSON(writer, http.StatusCreated, profile)
	default:
		writer.Header().Set("Allow", "GET, POST")
		writeError(writer, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

func (s *Server) profileResource(writer http.ResponseWriter, request *http.Request) {
	path := strings.Trim(strings.TrimPrefix(request.URL.Path, "/api/v1/profiles/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(writer, http.StatusNotFound, errors.New("profile not found"))
		return
	}
	id := parts[0]
	profile, ok := s.findProfile(id)
	if !ok || !scopeAllows(requestScope(request), profile) {
		writeError(writer, http.StatusNotFound, errors.New("profile not found"))
		return
	}
	if len(parts) == 2 && parts[1] == "rotate-endpoint" && request.Method == http.MethodPost {
		s.rotateEndpoint(writer, id)
		return
	}
	if len(parts) == 2 && parts[1] == "summary" && request.Method == http.MethodGet {
		writeJSON(writer, http.StatusOK, s.events.Summary(id))
		return
	}
	if len(parts) == 3 && parts[1] == "packages" && request.Method == http.MethodPut {
		s.updateProfilePackage(writer, request, id, parts[2])
		return
	}
	if len(parts) == 3 && parts[1] == "setup" && request.Method == http.MethodGet {
		s.serveSetupFile(writer, profile, parts[2])
		return
	}
	if len(parts) != 1 {
		writeError(writer, http.StatusNotFound, errors.New("resource not found"))
		return
	}

	switch request.Method {
	case http.MethodGet:
		writeJSON(writer, http.StatusOK, profile)
	case http.MethodPut:
		var input profileRequest
		if err := decodeJSON(request, &input); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		if input.DefaultAction == "" {
			input.DefaultAction = policy.ActionAllow
		}
		var updated policy.Profile
		err := s.update(func(cfg *config.Config) error {
			profile, ok := profileByID(cfg.Profiles, id)
			if !ok {
				return os.ErrNotExist
			}
			profile.Label = strings.TrimSpace(input.Label)
			if profile.Label == "" {
				return errors.New("label is required")
			}
			profile.DefaultAction = input.DefaultAction
			profile.Rules = append([]policy.Rule(nil), input.Rules...)
			if input.TimeZone != nil {
				zone := strings.TrimSpace(*input.TimeZone)
				if _, err := time.LoadLocation(zone); err != nil {
					return errors.New("fuso horário IANA inválido")
				}
				if profile.HouseID == "" {
					return errors.New("perfil não pertence a uma casa")
				}
				houseIndex := slices.IndexFunc(cfg.Houses, func(house config.House) bool { return house.ID == profile.HouseID })
				if houseIndex < 0 {
					return errors.New("casa do perfil não encontrada")
				}
				if cfg.Houses[houseIndex].TimeZone != zone {
					for index := range cfg.Profiles {
						if cfg.Profiles[index].HouseID == profile.HouseID && cfg.Profiles[index].ID != profile.ID {
							cfg.Profiles[index].Version++
						}
					}
				}
				cfg.Houses[houseIndex].TimeZone = zone
			}
			if input.Pauses != nil {
				profile.Pauses = cloneTimeWindows(*input.Pauses)
			}
			if input.Groups != nil {
				groups, err := mergeGroups(profile.Groups, input.Groups)
				if err != nil {
					return err
				}
				profile.Groups = groups
			}
			profile.Version++
			updated = *profile
			return nil
		})
		if errors.Is(err, os.ErrNotExist) {
			writeError(writer, http.StatusNotFound, errors.New("profile not found"))
			return
		}
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		writeJSON(writer, http.StatusOK, updated)
	case http.MethodDelete:
		var updated policy.Profile
		err := s.update(func(cfg *config.Config) error {
			profile, ok := profileByID(cfg.Profiles, id)
			if !ok {
				return os.ErrNotExist
			}
			profile.Disabled = true
			profile.Version++
			updated = *profile
			return nil
		})
		if errors.Is(err, os.ErrNotExist) {
			writeError(writer, http.StatusNotFound, errors.New("profile not found"))
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}
		writeJSON(writer, http.StatusOK, updated)
	default:
		writer.Header().Set("Allow", "GET, PUT, DELETE")
		writeError(writer, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

// serveSetupFile generates, on request, the per-profile device configuration
// for a kind in {info, windows.bat, windows-remove.bat, apple.mobileconfig}.
// The profile has already passed the house-scope check in profileResource;
// here we only refuse disabled profiles, whose endpoints can no longer be used.
func (s *Server) serveSetupFile(writer http.ResponseWriter, profile policy.Profile, kind string) {
	if profile.Disabled {
		writeError(writer, http.StatusNotFound, errors.New("perfil não encontrado"))
		return
	}
	switch kind {
	case "info":
		writer.Header().Set("Cache-Control", "no-store")
		writeJSON(writer, http.StatusOK, setupInfoResponse{
			Hostname: profile.Hostname, IP: s.resolverIP, Port: s.dnsPort, TestDomain: s.testDomain,
		})
		return
	case "windows.bat":
		content, err := setup.WindowsInstallBat(s.setupParams(profile))
		if err != nil {
			writeError(writer, http.StatusUnprocessableEntity, err)
			return
		}
		writeSetupFile(writer, setupWindowsMime, setupBatchFilename, content)
		return
	case "windows-remove.bat":
		content, err := setup.WindowsRemoveBat(s.setupParams(profile))
		if err != nil {
			writeError(writer, http.StatusUnprocessableEntity, err)
			return
		}
		writeSetupFile(writer, setupWindowsMime, setupRemoveFilename, content)
		return
	case "apple.mobileconfig":
		content, err := setup.AppleMobileConfig(s.setupParams(profile))
		if err != nil {
			writeError(writer, http.StatusUnprocessableEntity, err)
			return
		}
		writeSetupFile(writer, setupAppleMime, setupProfileFilename, string(content))
		return
	default:
		writeError(writer, http.StatusNotFound, errors.New("recurso de configuração não encontrado"))
	}
}

func (s *Server) setupParams(profile policy.Profile) setup.Params {
	return setup.Params{
		Hostname:   profile.Hostname,
		IP:         s.resolverIP,
		Port:       s.dnsPort,
		TestDomain: s.testDomain,
	}
}

// writeSetupFile streams a generated artifact as an attachment so the browser
// saves it with a stable, neutral filename.
func writeSetupFile(writer http.ResponseWriter, contentType, filename, content string) {
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(writer, content)
}

func (s *Server) updateProfilePackage(writer http.ResponseWriter, request *http.Request, profileID, packageID string) {
	var input packageRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	packages, err := availablePackageGroups(s.catalogDir)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	packageIndex := slices.IndexFunc(packages, func(group policy.RuleGroup) bool { return group.ID == packageID })
	if packageIndex < 0 {
		writeError(writer, http.StatusNotFound, errors.New("pacote não encontrado"))
		return
	}
	if input.Action == "" {
		input.Action = packages[packageIndex].Action
	}
	if !validPackageAction(input.Action) {
		writeError(writer, http.StatusBadRequest, errors.New("ação inválida"))
		return
	}
	var updated policy.Profile
	err = s.update(func(cfg *config.Config) error {
		profile, ok := profileByID(cfg.Profiles, profileID)
		if !ok {
			return os.ErrNotExist
		}
		index := slices.IndexFunc(profile.Groups, func(group policy.RuleGroup) bool { return group.ID == packageID })
		if !input.Enabled {
			if index >= 0 {
				profile.Groups = append(profile.Groups[:index], profile.Groups[index+1:]...)
			}
		} else if index >= 0 && len(profile.Groups[index].Domains) > 0 {
			profile.Groups[index].Action = input.Action
		} else {
			group := packages[packageIndex]
			group.Action = input.Action
			if index >= 0 {
				profile.Groups[index] = group
			} else {
				profile.Groups = append(profile.Groups, group)
			}
		}
		profile.Version++
		updated = *profile
		return nil
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	writeJSON(writer, http.StatusOK, updated)
}

func (s *Server) rotateEndpoint(writer http.ResponseWriter, id string) {
	token, err := randomToken()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	var updated policy.Profile
	err = s.update(func(cfg *config.Config) error {
		profile, ok := profileByID(cfg.Profiles, id)
		if !ok {
			return os.ErrNotExist
		}
		profile.Hostname = "p-" + token + "." + s.hostnameSuffix
		profile.Version++
		updated = *profile
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		writeError(writer, http.StatusNotFound, errors.New("profile not found"))
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeJSON(writer, http.StatusOK, updated)
}

func (s *Server) update(mutate func(*config.Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := cloneConfig(s.config)
	if err := mutate(&next); err != nil {
		return err
	}
	if err := config.ApplyHouseTimeZones(&next); err != nil {
		return err
	}
	if _, err := policy.NewStore(next.Profiles); err != nil {
		return fmt.Errorf("validate profiles: %w", err)
	}
	mode := os.FileMode(0o600)
	if info, err := os.Stat(s.configPath); err == nil {
		mode = info.Mode().Perm()
	}
	if err := config.WriteAtomic(s.configPath, next, mode); err != nil {
		return err
	}
	if err := s.profiles.Replace(next.Profiles); err != nil {
		return fmt.Errorf("activate profiles: %w", err)
	}
	s.config = next
	return nil
}

func (s *Server) findProfile(id string) (policy.Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, profile := range s.config.Profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return policy.Profile{}, false
}

func profileByID(profiles []policy.Profile, id string) (*policy.Profile, bool) {
	index := slices.IndexFunc(profiles, func(profile policy.Profile) bool { return profile.ID == id })
	if index < 0 {
		return nil, false
	}
	return &profiles[index], true
}

func mergeGroups(existing, incoming []policy.RuleGroup) ([]policy.RuleGroup, error) {
	known := make(map[string]policy.RuleGroup, len(existing))
	for _, group := range existing {
		known[group.ID] = group
	}
	result := make([]policy.RuleGroup, len(incoming))
	for index, group := range incoming {
		group.ID = strings.TrimSpace(group.ID)
		group.Name = strings.TrimSpace(group.Name)
		if group.ID == "" || group.Name == "" {
			return nil, fmt.Errorf("group %d requires id and name", index)
		}
		stored, exists := known[group.ID]
		if len(group.Domains) == 0 && !exists {
			return nil, fmt.Errorf("group %q requires at least one domain", group.Name)
		}
		if exists {
			group.DomainSource = stored.DomainSource
			group.DefaultDomains = append([]string(nil), stored.DefaultDomains...)
			if group.Schedules == nil {
				group.Schedules = cloneScheduledActions(stored.Schedules)
			} else {
				group.Schedules = cloneScheduledActions(group.Schedules)
			}
			if !group.Customized && len(group.DefaultDomains) > 0 {
				group.Domains = append([]string(nil), group.DefaultDomains...)
			}
		} else {
			group.DomainSource = ""
			group.DefaultDomains = nil
			group.Customized = true
		}
		result[index] = group
	}
	return result, nil
}

func (s *Server) authorize(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		provided := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if len(provided) == len(s.token) && subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) == 1 {
			next(writer, request.WithContext(context.WithValue(request.Context(), accessContextKey{}, accessScope{operator: true})))
			return
		}
		if houseID, ok := s.magicLinks.HouseID(provided); ok {
			next(writer, request.WithContext(context.WithValue(request.Context(), accessContextKey{}, accessScope{houseID: houseID})))
			return
		}
		providedHash := tokenHash(provided)
		s.mu.RLock()
		var houseID string
		for _, house := range s.config.Houses {
			if constantTimeEqual(house.AdminTokenHash, providedHash) {
				houseID = house.ID
			}
		}
		s.mu.RUnlock()
		if houseID == "" {
			writeError(writer, http.StatusUnauthorized, errors.New("invalid admin token"))
			return
		}
		next(writer, request.WithContext(context.WithValue(request.Context(), accessContextKey{}, accessScope{houseID: houseID})))
	}
}

func requestScope(request *http.Request) accessScope {
	scope, _ := request.Context().Value(accessContextKey{}).(accessScope)
	return scope
}

func scopeAllows(scope accessScope, profile policy.Profile) bool {
	return scope.operator || (scope.houseID != "" && profile.HouseID == scope.houseID)
}

func filterProfiles(profiles []policy.Profile, scope accessScope) []policy.Profile {
	result := make([]policy.Profile, 0, len(profiles))
	for _, profile := range profiles {
		if scopeAllows(scope, profile) {
			result = append(result, profile)
		}
	}
	return result
}

// findValidInvitation returns the first invitation whose hash matches
// wanted, is unused and has not expired. Comparison is constant-time so
// invitation codes cannot be recovered by timing.
func findValidInvitation(invitations []config.Invitation, wanted string, now time.Time) (config.Invitation, bool) {
	index, ok := findValidInvitationIndex(invitations, wanted, now)
	if !ok {
		return config.Invitation{}, false
	}
	return invitations[index], true
}

func findValidInvitationIndex(invitations []config.Invitation, wanted string, now time.Time) (int, bool) {
	for index, invitation := range invitations {
		if invitation.UsedAt != nil || now.After(invitation.ExpiresAt) {
			continue
		}
		if constantTimeEqual(invitation.CodeHash, wanted) {
			return index, true
		}
	}
	return 0, false
}

func constantTimeEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func environmentOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

type presetCatalog struct {
	Presets []presetDefinition `json:"presets"`
}

type presetDefinition struct {
	ID               string                   `json:"id"`
	Themes           map[string]policy.Action `json:"themes"`
	ServiceOverrides map[string]policy.Action `json:"service_overrides"`
}

type serviceCatalog struct {
	Services map[string]serviceDefinition `json:"services"`
}

type serviceDefinition struct {
	Label        string `json:"label"`
	Theme        string `json:"theme"`
	DomainSource string `json:"domain_source"`
}

func presetGroups(id, catalogDir string) ([]policy.RuleGroup, error) {
	if id == "" {
		id = "explorando"
	}
	var catalog presetCatalog
	if err := readCatalogJSON(filepath.Join(catalogDir, "presets.json"), &catalog); err != nil {
		return nil, err
	}
	var preset *presetDefinition
	for index := range catalog.Presets {
		if catalog.Presets[index].ID == id {
			preset = &catalog.Presets[index]
			break
		}
	}
	if preset == nil {
		return nil, errors.New("preset desconhecido")
	}
	groups, err := availablePackageGroups(catalogDir)
	if err != nil {
		return nil, err
	}
	for index := range groups {
		action, exists := preset.ServiceOverrides[strings.TrimPrefix(groups[index].ID, "service-")]
		if !exists {
			action, err = presetAction(preset.Themes, groups[index].Category)
			if err != nil {
				return nil, err
			}
		}
		groups[index].Action = action
	}
	return groups, nil
}

func availablePackageGroups(catalogDir string) ([]policy.RuleGroup, error) {
	type groupSeed struct {
		id, name, category, reason, file string
		action                           policy.Action
	}
	seeds := []groupSeed{
		{id: "gambling-br", name: "Apostas", category: "gambling", reason: "Apostas usam dinheiro real e podem criar hábitos difíceis de controlar", file: "gambling-br-authorized.txt", action: policy.ActionBlock},
		{id: "adult", name: "Conteúdo adulto", category: "adult_content", reason: "Conteúdo sexual explícito pede contexto, conversa e um acordo adequado para esta fase", file: "adult-content-regulators.txt", action: policy.ActionBlock},
	}
	groups := make([]policy.RuleGroup, 0, len(seeds)+16)
	for _, seed := range seeds {
		path := filepath.Join(catalogDir, seed.file)
		domains, err := config.LoadDomains(path)
		if err != nil {
			return nil, fmt.Errorf("carregar pacote %q: %w", seed.id, err)
		}
		groups = append(groups, policy.RuleGroup{
			ID: seed.id, Name: seed.name, Action: seed.action, Category: seed.category,
			Reason: seed.reason, Domains: append([]string{}, domains...),
			DefaultDomains: append([]string{}, domains...), DomainSource: path,
		})
	}
	var services serviceCatalog
	if err := readCatalogJSON(filepath.Join(catalogDir, "service-pools.json"), &services); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(services.Services))
	for serviceID := range services.Services {
		ids = append(ids, serviceID)
	}
	sort.Strings(ids)
	for _, serviceID := range ids {
		service := services.Services[serviceID]
		path := filepath.Join(catalogDir, service.DomainSource)
		domains, err := config.LoadDomains(path)
		if err != nil {
			return nil, fmt.Errorf("carregar serviço %q: %w", serviceID, err)
		}
		groups = append(groups, policy.RuleGroup{
			ID: "service-" + serviceID, Name: service.Label, Action: policy.ActionObserve, Category: service.Theme,
			Reason: serviceReason(service.Theme), Domains: append([]string{}, domains...),
			DefaultDomains: append([]string{}, domains...), DomainSource: path,
		})
	}
	return groups, nil
}

func validPackageAction(action policy.Action) bool {
	return action == policy.ActionAllow || action == policy.ActionBlock || action == policy.ActionObserve
}

func readCatalogJSON(path string, target any) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("ler catálogo %s: %w", path, err)
	}
	if err := json.Unmarshal(contents, target); err != nil {
		return fmt.Errorf("decodificar catálogo %s: %w", path, err)
	}
	return nil
}

func presetAction(themes map[string]policy.Action, theme string) (policy.Action, error) {
	action := themes[theme]
	switch action {
	case policy.ActionAllow, policy.ActionBlock, policy.ActionObserve:
		return action, nil
	default:
		return "", fmt.Errorf("ação ausente ou inválida para tema %q", theme)
	}
}

func serviceReason(theme string) string {
	switch theme {
	case "messaging_and_communities":
		return "Comunidades e mensagens também são espaços de amizade; o combinado deve preservar contato e segurança"
	case "games_and_social_play":
		return "Jogos online misturam brincadeira, criação, compras e contato com outras pessoas"
	case "social_video":
		return "Vídeo social mistura criação, aprendizado, publicidade e disputa por atenção"
	default:
		return "Redes sociais misturam convivência, entretenimento, publicidade e pressão por atenção"
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Vary", "Origin")
			writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func randomToken() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate endpoint token: %w", err)
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(value)), nil
}

func cloneConfig(cfg config.Config) config.Config {
	result := cfg
	result.Houses = make([]config.House, len(cfg.Houses))
	for index, house := range cfg.Houses {
		result.Houses[index] = house
		result.Houses[index].Emails = append([]string{}, house.Emails...)
	}
	result.Invitations = append([]config.Invitation{}, cfg.Invitations...)
	result.Waitlist = append([]config.WaitlistEntry{}, cfg.Waitlist...)
	result.Profiles = make([]policy.Profile, len(cfg.Profiles))
	for index, profile := range cfg.Profiles {
		result.Profiles[index] = profile
		result.Profiles[index].Rules = make([]policy.Rule, len(profile.Rules))
		copy(result.Profiles[index].Rules, profile.Rules)
		result.Profiles[index].Groups = make([]policy.RuleGroup, len(profile.Groups))
		for groupIndex, group := range profile.Groups {
			result.Profiles[index].Groups[groupIndex] = group
			result.Profiles[index].Groups[groupIndex].Domains = append([]string{}, group.Domains...)
			result.Profiles[index].Groups[groupIndex].DefaultDomains = append([]string{}, group.DefaultDomains...)
			result.Profiles[index].Groups[groupIndex].Schedules = cloneScheduledActions(group.Schedules)
		}
		result.Profiles[index].Pauses = cloneTimeWindows(profile.Pauses)
	}
	return result
}

func cloneTimeWindows(windows []policy.TimeWindow) []policy.TimeWindow {
	result := make([]policy.TimeWindow, len(windows))
	for index, window := range windows {
		result[index] = window
		result[index].Days = append([]int(nil), window.Days...)
	}
	return result
}

func cloneScheduledActions(schedules []policy.ScheduledAction) []policy.ScheduledAction {
	result := make([]policy.ScheduledAction, len(schedules))
	for index, schedule := range schedules {
		result[index] = schedule
		result[index].Days = append([]int(nil), schedule.Days...)
	}
	return result
}

func decodeJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, map[string]string{"error": err.Error(), "timestamp": time.Now().UTC().Format(time.RFC3339)})
}
