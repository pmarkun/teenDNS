// Package magiclink issues one-time login links for houses and exchanges
// them for short-lived sessions, without ever touching a house's permanent
// admin token. State lives in memory only, mirroring the shape of
// internal/pairing.Manager: a restart invalidates pending links and active
// sessions, but requesting a new link is immediate and the permanent
// token-paste login keeps working regardless.
package magiclink

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
	"sync"
	"time"
)

const maxPendingLinks = 10_000

type linkRecord struct {
	houseID   string
	expiresAt time.Time
}

type sessionRecord struct {
	houseID   string
	expiresAt time.Time
}

type Manager struct {
	mu         sync.Mutex
	linkTTL    time.Duration
	sessionTTL time.Duration
	now        func() time.Time
	links      map[string]linkRecord
	sessions   map[string]sessionRecord
}

func NewManager(linkTTL, sessionTTL time.Duration) *Manager {
	return &Manager{
		linkTTL:    linkTTL,
		sessionTTL: sessionTTL,
		now:        time.Now,
		links:      make(map[string]linkRecord),
		sessions:   make(map[string]sessionRecord),
	}
}

// IssueLink creates a one-time login token for houseID, valid for linkTTL.
func (m *Manager) IssueLink(houseID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanup()
	if len(m.links) >= maxPendingLinks {
		return "", errors.New("too many pending magic links")
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	m.links[token] = linkRecord{houseID: houseID, expiresAt: m.now().Add(m.linkTTL)}
	return token, nil
}

// Redeem consumes a one-time login token and mints a session token for the
// same house. The link token is removed immediately, so it cannot be
// redeemed a second time even if it has not expired yet.
func (m *Manager) Redeem(token string) (houseID, sessionToken string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanup()
	record, exists := m.links[token]
	if !exists {
		return "", "", false
	}
	delete(m.links, token)
	if m.now().After(record.expiresAt) {
		return "", "", false
	}
	sessionToken, err := randomToken()
	if err != nil {
		return "", "", false
	}
	m.sessions[sessionToken] = sessionRecord{houseID: record.houseID, expiresAt: m.now().Add(m.sessionTTL)}
	return record.houseID, sessionToken, true
}

// HouseID resolves an active session token back to the house it was issued
// for. Used by the admin API's authorize middleware alongside the
// permanent per-house token check.
func (m *Manager) HouseID(sessionToken string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanup()
	session, ok := m.sessions[sessionToken]
	if !ok {
		return "", false
	}
	return session.houseID, true
}

func (m *Manager) cleanup() {
	now := m.now()
	for token, record := range m.links {
		if now.After(record.expiresAt) {
			delete(m.links, token)
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
