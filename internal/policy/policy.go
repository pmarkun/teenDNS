package policy

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
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

type RuleGroup struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Action         Action            `json:"action"`
	Category       string            `json:"category,omitempty"`
	Reason         string            `json:"reason,omitempty"`
	Domains        []string          `json:"domains"`
	DefaultDomains []string          `json:"default_domains,omitempty"`
	DomainSource   string            `json:"domain_source,omitempty"`
	Customized     bool              `json:"customized,omitempty"`
	Schedules      []ScheduledAction `json:"schedules,omitempty"`
}

type TimeWindow struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Days  []int  `json:"days"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type ScheduledAction struct {
	TimeWindow
	Action Action `json:"action"`
}

type Profile struct {
	ID            string         `json:"id"`
	HouseID       string         `json:"house_id,omitempty"`
	Label         string         `json:"label,omitempty"`
	Hostname      string         `json:"hostname"`
	Disabled      bool           `json:"disabled,omitempty"`
	DefaultAction Action         `json:"default_action"`
	Version       int64          `json:"version"`
	Rules         []Rule         `json:"rules"`
	Groups        []RuleGroup    `json:"groups,omitempty"`
	Pauses        []TimeWindow   `json:"pauses,omitempty"`
	TimeZone      string         `json:"-"`
	Location      *time.Location `json:"-"`
}

type Decision struct {
	Action        Action
	Category      string
	Reason        string
	MatchedDomain string
	GroupName     string
	ScheduleLabel string
	PolicyVersion int64
}

type Store struct {
	profiles      map[string]Profile
	profileLabels map[string]Profile
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

func (m *Manager) ProfileLabel(label string) (Profile, bool) {
	return m.store.Load().ProfileLabel(label)
}

func NewStore(profiles []Profile) (*Store, error) {
	store := &Store{
		profiles:      make(map[string]Profile, len(profiles)),
		profileLabels: make(map[string]Profile, len(profiles)),
	}

	for _, profile := range profiles {
		if profile.Disabled {
			continue
		}
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
		profileLabel := strings.SplitN(normalizedHostname, ".", 2)[0]
		if _, exists := store.profileLabels[profileLabel]; exists {
			return nil, fmt.Errorf("duplicate profile endpoint label %q", profileLabel)
		}
		if !validAction(profile.DefaultAction) {
			return nil, fmt.Errorf("profile %q has invalid default action %q", profile.ID, profile.DefaultAction)
		}
		location, err := profileLocation(profile.TimeZone)
		if err != nil {
			return nil, fmt.Errorf("profile %q time zone: %w", profile.ID, err)
		}
		profile.Location = location
		pauseIDs := make(map[string]struct{}, len(profile.Pauses))
		for index, pause := range profile.Pauses {
			if err := validateWindow(pause); err != nil {
				return nil, fmt.Errorf("profile %q pause %d: %w", profile.ID, index, err)
			}
			if _, exists := pauseIDs[pause.ID]; exists {
				return nil, fmt.Errorf("profile %q repeats pause id %q", profile.ID, pause.ID)
			}
			pauseIDs[pause.ID] = struct{}{}
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
		groupIDs := make(map[string]struct{}, len(profile.Groups))
		for index := range profile.Groups {
			group := &profile.Groups[index]
			if group.ID == "" || group.Name == "" {
				return nil, fmt.Errorf("profile %q group %d requires id and name", profile.ID, index)
			}
			if _, exists := groupIDs[group.ID]; exists {
				return nil, fmt.Errorf("profile %q has duplicate group id %q", profile.ID, group.ID)
			}
			groupIDs[group.ID] = struct{}{}
			if !validAction(group.Action) {
				return nil, fmt.Errorf("profile %q group %q has invalid action %q", profile.ID, group.ID, group.Action)
			}
			scheduleIDs := make(map[string]struct{}, len(group.Schedules))
			occupied := make(map[int]string)
			for scheduleIndex, schedule := range group.Schedules {
				if err := validateWindow(schedule.TimeWindow); err != nil {
					return nil, fmt.Errorf("profile %q group %q schedule %d: %w", profile.ID, group.ID, scheduleIndex, err)
				}
				if schedule.Action != ActionAllow && schedule.Action != ActionBlock {
					return nil, fmt.Errorf("profile %q group %q schedule %q action must be allow or block", profile.ID, group.ID, schedule.ID)
				}
				if _, exists := scheduleIDs[schedule.ID]; exists {
					return nil, fmt.Errorf("profile %q group %q repeats schedule id %q", profile.ID, group.ID, schedule.ID)
				}
				scheduleIDs[schedule.ID] = struct{}{}
				if conflict := occupyWindow(occupied, schedule.TimeWindow); conflict != "" {
					return nil, fmt.Errorf("profile %q group %q schedules %q and %q overlap", profile.ID, group.ID, conflict, schedule.ID)
				}
			}
			seenDomains := make(map[string]struct{}, len(group.Domains))
			for domainIndex, domain := range group.Domains {
				normalized, err := normalizeName(domain)
				if err != nil {
					return nil, fmt.Errorf("profile %q group %q domain %d: %w", profile.ID, group.ID, domainIndex, err)
				}
				if _, exists := seenDomains[normalized]; exists {
					return nil, fmt.Errorf("profile %q group %q repeats domain %q", profile.ID, group.ID, normalized)
				}
				seenDomains[normalized] = struct{}{}
				group.Domains[domainIndex] = normalized
			}
		}
		store.profiles[normalizedHostname] = profile
		store.profileLabels[profileLabel] = profile
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

func (s *Store) ProfileLabel(label string) (Profile, bool) {
	normalized, err := normalizeName(label)
	if err != nil || strings.Contains(normalized, ".") {
		return Profile{}, false
	}
	profile, ok := s.profileLabels[normalized]
	return profile, ok
}

func Decide(profile Profile, queryName string) (Decision, error) {
	return DecideAt(profile, queryName, time.Now())
}

func DecideAt(profile Profile, queryName string, now time.Time) (Decision, error) {
	name, err := normalizeName(queryName)
	if err != nil {
		return Decision{}, err
	}

	decision := Decision{
		Action:        profile.DefaultAction,
		PolicyVersion: profile.Version,
	}
	location := profile.Location
	if location == nil {
		location, err = profileLocation(profile.TimeZone)
		if err != nil {
			return Decision{}, err
		}
	}
	localNow := now.In(location)
	for _, pause := range profile.Pauses {
		if windowActive(pause, localNow) {
			decision.Action = ActionBlock
			decision.Category = "global_pause"
			decision.Reason = "Pausa geral"
			decision.ScheduleLabel = pause.Label
			return decision, nil
		}
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
	for _, group := range profile.Groups {
		for _, domain := range group.Domains {
			matches := name == domain || strings.HasSuffix(name, "."+domain)
			if !matches || len(domain) <= bestMatchLength {
				continue
			}
			bestMatchLength = len(domain)
			decision.Action = group.Action
			decision.Category = group.Category
			decision.Reason = group.Reason
			decision.MatchedDomain = domain
			decision.GroupName = group.Name
			if schedule, active := activeScheduledAction(group.Schedules, localNow); active {
				decision.Action = schedule.Action
				decision.ScheduleLabel = schedule.Label
			}
		}
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
