package pairing

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
	"sync"
	"time"
)

const maxPendingChallenges = 10_000

type Challenge struct {
	ID        string    `json:"id"`
	DNSName   string    `json:"dns_name"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Status struct {
	Paired       bool      `json:"paired"`
	SessionToken string    `json:"session_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

type challengeRecord struct {
	dnsToken      string
	expiresAt     time.Time
	profileID     string
	sessionToken  string
	sessionExpiry time.Time
}

type sessionRecord struct {
	profileID string
	expiresAt time.Time
}

type Manager struct {
	mu           sync.Mutex
	suffix       string
	challengeTTL time.Duration
	sessionTTL   time.Duration
	now          func() time.Time
	challenges   map[string]*challengeRecord
	dnsTokens    map[string]string
	sessions     map[string]sessionRecord
}

func NewManager(suffix string, challengeTTL, sessionTTL time.Duration) *Manager {
	return &Manager{
		suffix:       strings.Trim(strings.ToLower(suffix), "."),
		challengeTTL: challengeTTL,
		sessionTTL:   sessionTTL,
		now:          time.Now,
		challenges:   make(map[string]*challengeRecord),
		dnsTokens:    make(map[string]string),
		sessions:     make(map[string]sessionRecord),
	}
}

func (m *Manager) Create() (Challenge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanup()
	if len(m.challenges) >= maxPendingChallenges {
		return Challenge{}, errors.New("too many pending pairing challenges")
	}
	id, err := randomToken()
	if err != nil {
		return Challenge{}, err
	}
	dnsToken, err := randomToken()
	if err != nil {
		return Challenge{}, err
	}
	expiresAt := m.now().Add(m.challengeTTL)
	m.challenges[id] = &challengeRecord{dnsToken: dnsToken, expiresAt: expiresAt}
	m.dnsTokens[dnsToken] = id
	return Challenge{ID: id, DNSName: dnsToken + "." + m.suffix, ExpiresAt: expiresAt}, nil
}

// Observe records that a one-time DNS challenge was resolved through a profile.
// It returns true only for an active challenge owned by this manager.
func (m *Manager) Observe(profileID, queryName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanup()
	name := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(queryName)), ".")
	suffix := "." + m.suffix
	if !strings.HasSuffix(name, suffix) {
		return false
	}
	dnsToken := strings.TrimSuffix(name, suffix)
	if dnsToken == "" || strings.Contains(dnsToken, ".") {
		return false
	}
	id, ok := m.dnsTokens[dnsToken]
	if !ok {
		return false
	}
	record := m.challenges[id]
	if record == nil {
		return false
	}
	if record.profileID == "" {
		sessionToken, err := randomToken()
		if err != nil {
			return false
		}
		record.profileID = profileID
		record.sessionToken = sessionToken
		record.sessionExpiry = m.now().Add(m.sessionTTL)
		m.sessions[sessionToken] = sessionRecord{profileID: profileID, expiresAt: record.sessionExpiry}
	}
	return true
}

func (m *Manager) Status(id string) (Status, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanup()
	record, ok := m.challenges[id]
	if !ok {
		return Status{}, false
	}
	if record.profileID == "" {
		return Status{Paired: false}, true
	}
	return Status{Paired: true, SessionToken: record.sessionToken, ExpiresAt: record.sessionExpiry}, true
}

// Outcome reports the profile that resolved a challenge's DNS name, once it
// has been observed. The second return value is false when the challenge is
// unknown or expired. Unlike Status, it never returns a session token, so it
// is safe for the responsible side of the flow (the panel).
func (m *Manager) Outcome(id string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanup()
	record, ok := m.challenges[id]
	if !ok {
		return "", false
	}
	if record.profileID == "" {
		return "", true
	}
	return record.profileID, true
}

func (m *Manager) Profile(sessionToken string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanup()
	session, ok := m.sessions[sessionToken]
	if !ok {
		return "", false
	}
	return session.profileID, true
}

func (m *Manager) cleanup() {
	now := m.now()
	for id, record := range m.challenges {
		if now.After(record.expiresAt) {
			delete(m.dnsTokens, record.dnsToken)
			delete(m.challenges, id)
		}
	}
	for token, session := range m.sessions {
		if now.After(session.expiresAt) {
			delete(m.sessions, token)
		}
	}
}

func randomToken() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(value)), nil
}
