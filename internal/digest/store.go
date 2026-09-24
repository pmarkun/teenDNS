// Package digest accumulates domains observed under the "observe" policy
// action, bucketed by day part instead of exact timestamp, and turns them
// into a periodic email for the responsible party. It deliberately never
// keeps a browsing sequence: only (profile, day part, matched catalog
// domain) counters, purged after each successful send.
package digest

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	_ "time/tzdata" // guarantees LoadLocation works even on minimal base images without system tzdata

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/gateway"
	"github.com/pmarkun/teendns/internal/policy"
)

const (
	zoneName = "America/Sao_Paulo"

	PeriodMorning   = "manha"
	PeriodAfternoon = "tarde"
	PeriodEvening   = "noite"
)

type key struct {
	ProfileID string
	Period    string
	Domain    string
}

type entry struct {
	Category  string
	Reason    string
	GroupName string
	Count     int
}

type snapshotEntry struct {
	ProfileID string `json:"profile_id"`
	Period    string `json:"period"`
	Domain    string `json:"domain"`
	Category  string `json:"category,omitempty"`
	Reason    string `json:"reason,omitempty"`
	GroupName string `json:"group_name,omitempty"`
	Count     int    `json:"count"`
}

type snapshot struct {
	Entries    []snapshotEntry      `json:"entries"`
	LastSentAt map[string]time.Time `json:"last_sent_at,omitempty"`
}

// Store implements gateway.EventSink and keeps the accumulated, privacy
// bounded observation state for the weekly digest.
type Store struct {
	mu         sync.Mutex
	path       string
	location   *time.Location
	entries    map[key]*entry
	lastSentAt map[string]time.Time
}

// NewStore loads any existing snapshot at path (a missing file is not an
// error) and returns a ready Store.
func NewStore(path string) (*Store, error) {
	location, err := time.LoadLocation(zoneName)
	if err != nil {
		return nil, err
	}
	store := &Store{
		path:       path,
		location:   location,
		entries:    make(map[key]*entry),
		lastSentAt: make(map[string]time.Time),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) load() error {
	contents, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var loaded snapshot
	if err := json.Unmarshal(contents, &loaded); err != nil {
		return err
	}
	for _, item := range loaded.Entries {
		s.entries[key{ProfileID: item.ProfileID, Period: item.Period, Domain: item.Domain}] = &entry{
			Category:  item.Category,
			Reason:    item.Reason,
			GroupName: item.GroupName,
			Count:     item.Count,
		}
	}
	for houseID, when := range loaded.LastSentAt {
		s.lastSentAt[houseID] = when
	}
	return nil
}

// Save persists the current state atomically. Production code normally goes
// through CompleteDigest instead; this is exposed for tests and for a
// periodic safety-net flush.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	current := snapshot{LastSentAt: s.lastSentAt}
	for storeKey, item := range s.entries {
		current.Entries = append(current.Entries, snapshotEntry{
			ProfileID: storeKey.ProfileID,
			Period:    storeKey.Period,
			Domain:    storeKey.Domain,
			Category:  item.Category,
			Reason:    item.Reason,
			GroupName: item.GroupName,
			Count:     item.Count,
		})
	}
	return config.WriteAtomic(s.path, current, 0o600)
}

// Write implements gateway.EventSink. It only records events with the
// "observe" action and only when a specific catalog domain matched — the
// raw query name is never used as a key, which keeps cardinality bounded to
// the catalog size per profile instead of every CDN/tracking subdomain seen
// on the wire.
func (s *Store) Write(event gateway.Event) {
	if event.Action != policy.ActionObserve || event.MatchedDomain == "" {
		return
	}
	period := periodFor(event.Timestamp, s.location)

	s.mu.Lock()
	defer s.mu.Unlock()
	storeKey := key{ProfileID: event.ProfileID, Period: period, Domain: event.MatchedDomain}
	item := s.entries[storeKey]
	if item == nil {
		item = &entry{}
		s.entries[storeKey] = item
	}
	item.Count++
	item.Category = event.Category
	item.Reason = event.Reason
	item.GroupName = event.GroupName
}

func periodFor(instant time.Time, location *time.Location) string {
	hour := instant.In(location).Hour()
	switch {
	case hour >= 5 && hour < 12:
		return PeriodMorning
	case hour >= 12 && hour < 18:
		return PeriodAfternoon
	default:
		return PeriodEvening
	}
}

// DomainPeriods lists how many times a domain was observed per day part.
type DomainPeriods struct {
	Domain string
	Counts map[string]int
}

// CategoryDigest groups observed domains under one category/group, carrying
// the human-written Reason to use as a conversation starter.
type CategoryDigest struct {
	Category  string
	GroupName string
	Reason    string
	Domains   []DomainPeriods
}

// Digest builds a report for the given profiles, grouped by category. The
// order is deterministic (sorted by category, then domain) so templates and
// tests are stable.
func (s *Store) Digest(profileIDs []string) []CategoryDigest {
	wanted := make(map[string]struct{}, len(profileIDs))
	for _, id := range profileIDs {
		wanted[id] = struct{}{}
	}

	type accumulator struct {
		groupName string
		reason    string
		domains   map[string]*DomainPeriods
	}

	s.mu.Lock()
	byCategory := make(map[string]*accumulator)
	for storeKey, item := range s.entries {
		if _, ok := wanted[storeKey.ProfileID]; !ok {
			continue
		}
		bucket := byCategory[item.Category]
		if bucket == nil {
			bucket = &accumulator{groupName: item.GroupName, reason: item.Reason, domains: make(map[string]*DomainPeriods)}
			byCategory[item.Category] = bucket
		}
		domain := bucket.domains[storeKey.Domain]
		if domain == nil {
			domain = &DomainPeriods{Domain: storeKey.Domain, Counts: make(map[string]int)}
			bucket.domains[storeKey.Domain] = domain
		}
		domain.Counts[storeKey.Period] += item.Count
	}
	s.mu.Unlock()

	result := make([]CategoryDigest, 0, len(byCategory))
	for category, bucket := range byCategory {
		digestEntry := CategoryDigest{Category: category, GroupName: bucket.groupName, Reason: bucket.reason}
		for _, domain := range bucket.domains {
			digestEntry.Domains = append(digestEntry.Domains, *domain)
		}
		result = append(result, digestEntry)
	}
	sortCategoryDigests(result)
	return result
}

// IsDue reports whether interval has elapsed since the last successful send
// for houseID. A house that never received a digest is always due.
func (s *Store) IsDue(houseID string, now time.Time, interval time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	last, ok := s.lastSentAt[houseID]
	if !ok {
		return true
	}
	return now.Sub(last) >= interval
}

// CompleteDigest clears the accumulated entries for the given profiles and
// records the send time for houseID, then persists the result. Call this
// only after the email was sent successfully.
func (s *Store) CompleteDigest(houseID string, profileIDs []string, when time.Time) error {
	wanted := make(map[string]struct{}, len(profileIDs))
	for _, id := range profileIDs {
		wanted[id] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for storeKey := range s.entries {
		if _, ok := wanted[storeKey.ProfileID]; ok {
			delete(s.entries, storeKey)
		}
	}
	s.lastSentAt[houseID] = when
	return s.saveLocked()
}

// Forget removes accumulated entries and last-sent bookkeeping for a house
// that is being deleted, so no observation data survives the account.
func (s *Store) Forget(houseID string, profileIDs []string) error {
	wanted := make(map[string]struct{}, len(profileIDs))
	for _, id := range profileIDs {
		wanted[id] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for storeKey := range s.entries {
		if _, ok := wanted[storeKey.ProfileID]; ok {
			delete(s.entries, storeKey)
		}
	}
	delete(s.lastSentAt, houseID)
	return s.saveLocked()
}
