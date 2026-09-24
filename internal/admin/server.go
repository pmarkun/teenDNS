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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/gateway"
	"github.com/pmarkun/teendns/internal/pairing"
	"github.com/pmarkun/teendns/internal/policy"
)

type Server struct {
	mu             sync.RWMutex
	configPath     string
	config         config.Config
	profiles       *policy.Manager
	events         *gateway.EventBuffer
	pairings       *pairing.Manager
	hostnameSuffix string
	token          string
	invitationKeys []string
	catalogDir     string
}

type accessScope struct {
	operator bool
	houseID  string
}

type accessContextKey struct{}

type profileRequest struct {
	Label         string             `json:"label"`
	DefaultAction policy.Action      `json:"default_action"`
	Rules         []policy.Rule      `json:"rules"`
	Groups        []policy.RuleGroup `json:"groups"`
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

type youthRule struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type youthProfile struct {
	Label string      `json:"label"`
	Rules []youthRule `json:"rules"`
}

func NewServer(configPath string, cfg config.Config, profiles *policy.Manager, events *gateway.EventBuffer, pairings *pairing.Manager, hostnameSuffix, token string) (*Server, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("admin token is required")
	}
	if pairings == nil {
		return nil, errors.New("pairing manager is required")
	}
	if hostnameSuffix == "" {
		hostnameSuffix = "dns.teendns.test"
	}
	return &Server{
		configPath:     configPath,
		config:         cloneConfig(cfg),
		profiles:       profiles,
		events:         events,
		pairings:       pairings,
		hostnameSuffix: strings.TrimSuffix(strings.ToLower(hostnameSuffix), "."),
		token:          token,
		invitationKeys: splitInvitationKeys(os.Getenv("TEENDNS_INVITATION_CODES")),
		catalogDir:     environmentOrDefault("TEENDNS_CATALOG_DIR", "catalog/v1"),
	}, nil
}

// Reload keeps the control plane in sync when an operator uses the legacy
// SIGHUP configuration path.
func (s *Server) Reload(cfg config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cloneConfig(cfg)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/v1/pairing/challenges", s.createPairingChallenge)
	mux.HandleFunc("GET /api/v1/pairing/challenges/{id}", s.pairingChallengeStatus)
	mux.HandleFunc("GET /api/v1/youth/profile", s.youthProfile)
	mux.HandleFunc("POST /api/v1/houses", s.registerHouse)
	mux.HandleFunc("/api/v1/profiles", s.authorize(s.profilesCollection))
	mux.HandleFunc("/api/v1/profiles/", s.authorize(s.profileResource))
	return withCORS(mux)
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
	if !containsConstantTime(s.invitationKeys, invitationKey) {
		writeError(writer, http.StatusForbidden, errors.New("convite inválido ou já usado"))
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
	house := config.House{ID: houseIDToken[:12], Name: input.HouseName, AdminTokenHash: tokenHash(houseToken)}
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
		if slices.Contains(cfg.UsedInvitationKeys, invitationKey) {
			return errors.New("convite inválido ou já usado")
		}
		cfg.UsedInvitationKeys = append(cfg.UsedInvitationKeys, invitationKey)
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
		if len(group.Domains) == 0 {
			return nil, fmt.Errorf("group %q requires at least one domain", group.Name)
		}
		if stored, ok := known[group.ID]; ok {
			group.DomainSource = stored.DomainSource
			group.DefaultDomains = append([]string(nil), stored.DefaultDomains...)
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

func splitInvitationKeys(value string) []string {
	keys := []string{}
	for _, code := range strings.Split(value, ",") {
		if code = strings.TrimSpace(code); code != "" {
			keys = append(keys, tokenHash(code))
		}
	}
	return keys
}

func containsConstantTime(values []string, wanted string) bool {
	found := 0
	for _, value := range values {
		found |= subtle.ConstantTimeCompare([]byte(value), []byte(wanted))
	}
	return found == 1
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

type presetDefinition struct {
	id             string
	socialAction   policy.Action
	adultAction    policy.Action
	gamblingAction policy.Action
}

func presetGroups(id, catalogDir string) ([]policy.RuleGroup, error) {
	if id == "" {
		id = "exploring"
	}
	presets := map[string]presetDefinition{
		"accompanied": {id: "accompanied", socialAction: policy.ActionBlock, adultAction: policy.ActionBlock, gamblingAction: policy.ActionBlock},
		"exploring":   {id: "exploring", socialAction: policy.ActionObserve, adultAction: policy.ActionBlock, gamblingAction: policy.ActionBlock},
		"guided":      {id: "guided", socialAction: policy.ActionObserve, adultAction: policy.ActionObserve, gamblingAction: policy.ActionBlock},
	}
	preset, ok := presets[id]
	if !ok {
		return nil, errors.New("preset desconhecido")
	}
	type groupSeed struct {
		id, name, category, reason, file string
		action                           policy.Action
	}
	seeds := []groupSeed{
		{id: "gambling-br", name: "Apostas", category: "gambling", reason: "Apostas usam dinheiro real e podem criar hábitos difíceis de controlar", file: "gambling-br-authorized.txt", action: preset.gamblingAction},
		{id: "adult", name: "Conteúdo adulto", category: "adult", reason: "Conteúdo sexual explícito pede contexto, conversa e um acordo adequado para esta fase", file: "adult-content-regulators.txt", action: preset.adultAction},
		{id: "social", name: "Redes sociais", category: "social", reason: "Redes sociais misturam convivência, entretenimento, publicidade e pressão por atenção", file: "social-platforms.txt", action: preset.socialAction},
	}
	groups := make([]policy.RuleGroup, 0, len(seeds))
	for _, seed := range seeds {
		path := filepath.Join(catalogDir, seed.file)
		domains, err := config.LoadDomains(path)
		if err != nil {
			return nil, fmt.Errorf("carregar preset %q: %w", preset.id, err)
		}
		groups = append(groups, policy.RuleGroup{
			ID: seed.id, Name: seed.name, Action: seed.action, Category: seed.category,
			Reason: seed.reason, Domains: append([]string{}, domains...),
			DefaultDomains: append([]string{}, domains...), DomainSource: path,
		})
	}
	return groups, nil
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
	result.Houses = append([]config.House{}, cfg.Houses...)
	result.UsedInvitationKeys = append([]string{}, cfg.UsedInvitationKeys...)
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
		}
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
