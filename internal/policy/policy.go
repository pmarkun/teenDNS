package policy

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
)

type Action string

const (
	ActionAllow   Action = "allow"
	ActionBlock   Action = "block"
	ActionObserve Action = "observe"
)

type Rule struct {
	Domain            string `json:"domain"`
	IncludeSubdomains bool   `json:"include_subdomains"`
	Action            Action `json:"action"`
	Category          string `json:"category,omitempty"`
	Reason            string `json:"reason,omitempty"`
}

type Profile struct {
	ID            string `json:"id"`
	Hostname      string `json:"hostname"`
	DefaultAction Action `json:"default_action"`
	Version       int64  `json:"version"`
	Rules         []Rule `json:"rules"`
}

type Decision struct {
	Action        Action
	Category      string
	Reason        string
	MatchedDomain string
	PolicyVersion int64
}

type Store struct {
	profiles map[string]Profile
}

type Manager struct {
	store atomic.Pointer[Store]
}

func NewManager(profiles []Profile) (*Manager, error) {
	store, err := NewStore(profiles)
	if err != nil {
		return nil, err
	}
	manager := &Manager{}
	manager.store.Store(store)
	return manager, nil
}

func (m *Manager) Replace(profiles []Profile) error {
	store, err := NewStore(profiles)
	if err != nil {
		return err
	}
	m.store.Store(store)
	return nil
}

func (m *Manager) Profile(hostname string) (Profile, bool) {
	return m.store.Load().Profile(hostname)
}

func NewStore(profiles []Profile) (*Store, error) {
	store := &Store{profiles: make(map[string]Profile, len(profiles))}

	for _, profile := range profiles {
		normalizedHostname, err := normalizeName(profile.Hostname)
		if err != nil {
			return nil, fmt.Errorf("profile %q hostname: %w", profile.ID, err)
		}
		if profile.ID == "" {
			return nil, errors.New("profile ID is required")
		}
		if _, exists := store.profiles[normalizedHostname]; exists {
			return nil, fmt.Errorf("duplicate profile hostname %q", normalizedHostname)
		}
		if !validAction(profile.DefaultAction) {
			return nil, fmt.Errorf("profile %q has invalid default action %q", profile.ID, profile.DefaultAction)
		}

		profile.Hostname = normalizedHostname
		for index := range profile.Rules {
			rule := &profile.Rules[index]
			rule.Domain, err = normalizeName(rule.Domain)
			if err != nil {
				return nil, fmt.Errorf("profile %q rule %d: %w", profile.ID, index, err)
			}
			if !validAction(rule.Action) {
				return nil, fmt.Errorf("profile %q rule %d has invalid action %q", profile.ID, index, rule.Action)
			}
		}
		store.profiles[normalizedHostname] = profile
	}

	return store, nil
}

func (s *Store) Profile(hostname string) (Profile, bool) {
	normalized, err := normalizeName(hostname)
	if err != nil {
		return Profile{}, false
	}
	profile, ok := s.profiles[normalized]
	return profile, ok
}

func Decide(profile Profile, queryName string) (Decision, error) {
	name, err := normalizeName(queryName)
	if err != nil {
		return Decision{}, err
	}

	decision := Decision{
		Action:        profile.DefaultAction,
		PolicyVersion: profile.Version,
	}
	bestMatchLength := -1

	for _, rule := range profile.Rules {
		matches := name == rule.Domain
		if rule.IncludeSubdomains {
			matches = matches || strings.HasSuffix(name, "."+rule.Domain)
		}
		if !matches || len(rule.Domain) <= bestMatchLength {
			continue
		}

		bestMatchLength = len(rule.Domain)
		decision.Action = rule.Action
		decision.Category = rule.Category
		decision.Reason = rule.Reason
		decision.MatchedDomain = rule.Domain
	}

	return decision, nil
}

func normalizeName(value string) (string, error) {
	value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if value == "" {
		return "", errors.New("domain name is required")
	}
	if strings.ContainsAny(value, " /:@") {
		return "", fmt.Errorf("invalid domain name %q", value)
	}
	return value, nil
}

func validAction(action Action) bool {
	switch action {
	case ActionAllow, ActionBlock, ActionObserve:
		return true
	default:
		return false
	}
}
