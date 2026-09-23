package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/gateway"
	"github.com/pmarkun/teendns/internal/policy"
)

type Server struct {
	mu             sync.RWMutex
	configPath     string
	config         config.Config
	profiles       *policy.Manager
	events         *gateway.EventBuffer
	hostnameSuffix string
	token          string
}

type profileRequest struct {
	Label         string        `json:"label"`
	DefaultAction policy.Action `json:"default_action"`
	Rules         []policy.Rule `json:"rules"`
}

func NewServer(configPath string, cfg config.Config, profiles *policy.Manager, events *gateway.EventBuffer, hostnameSuffix, token string) (*Server, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("admin token is required")
	}
	if hostnameSuffix == "" {
		hostnameSuffix = "dns.teendns.test"
	}
	return &Server{
		configPath:     configPath,
		config:         cloneConfig(cfg),
		profiles:       profiles,
		events:         events,
		hostnameSuffix: strings.TrimSuffix(strings.ToLower(hostnameSuffix), "."),
		token:          token,
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/v1/profiles", s.authorize(s.profilesCollection))
	mux.HandleFunc("/api/v1/profiles/", s.authorize(s.profileResource))
	return withCORS(mux)
}

func (s *Server) profilesCollection(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		s.mu.RLock()
		profiles := append([]policy.Profile(nil), s.config.Profiles...)
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
			Label:         input.Label,
			Hostname:      "p-" + token + "." + s.hostnameSuffix,
			DefaultAction: policy.ActionAllow,
			Version:       1,
			Rules:         []policy.Rule{},
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
		profile, ok := s.findProfile(id)
		if !ok {
			writeError(writer, http.StatusNotFound, errors.New("profile not found"))
			return
		}
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

func (s *Server) authorize(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		provided := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if len(provided) != len(s.token) || subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
			writeError(writer, http.StatusUnauthorized, errors.New("invalid admin token"))
			return
		}
		next(writer, request)
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
	result.Profiles = make([]policy.Profile, len(cfg.Profiles))
	for index, profile := range cfg.Profiles {
		result.Profiles[index] = profile
		result.Profiles[index].Rules = append([]policy.Rule(nil), profile.Rules...)
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
