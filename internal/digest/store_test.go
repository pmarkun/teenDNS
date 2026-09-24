package digest

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/pmarkun/teendns/internal/gateway"
	"github.com/pmarkun/teendns/internal/policy"
)

func mustLoadLocation(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(zoneName)
	if err != nil {
		t.Fatalf("load zone: %v", err)
	}
	return location
}

func TestPeriodForBoundaries(t *testing.T) {
	location := mustLoadLocation(t)
	cases := []struct {
		hour int
		want string
	}{
		{4, PeriodEvening},
		{5, PeriodMorning},
		{11, PeriodMorning},
		{12, PeriodAfternoon},
		{17, PeriodAfternoon},
		{18, PeriodEvening},
		{23, PeriodEvening},
	}
	for _, testCase := range cases {
		instant := time.Date(2026, 3, 10, testCase.hour, 0, 0, 0, location)
		if got := periodFor(instant, location); got != testCase.want {
			t.Fatalf("hour %d: expected %q, got %q", testCase.hour, testCase.want, got)
		}
	}
}

func TestWriteIgnoresNonObserveAndUnmatchedEvents(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.Write(gateway.Event{ProfileID: "p1", Action: policy.ActionBlock, MatchedDomain: "bet.test", Timestamp: time.Now()})
	store.Write(gateway.Event{ProfileID: "p1", Action: policy.ActionObserve, MatchedDomain: "", Timestamp: time.Now()})

	if report := store.Digest([]string{"p1"}); len(report) != 0 {
		t.Fatalf("expected no digest entries, got %+v", report)
	}
}

func TestWriteAndDigestAggregatesByDomainAndPeriod(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	location := mustLoadLocation(t)
	morning := time.Date(2026, 3, 10, 8, 0, 0, 0, location)
	evening := time.Date(2026, 3, 10, 20, 0, 0, 0, location)

	event := gateway.Event{
		ProfileID:     "p1",
		Action:        policy.ActionObserve,
		Category:      "messaging_and_communities",
		Reason:        "Comunidades e mensagens também são espaços de amizade",
		GroupName:     "Discord",
		MatchedDomain: "discord.com",
	}
	morningEvent := event
	morningEvent.Timestamp = morning
	eveningEvent := event
	eveningEvent.Timestamp = evening

	store.Write(morningEvent)
	store.Write(morningEvent)
	store.Write(eveningEvent)

	report := store.Digest([]string{"p1"})
	if len(report) != 1 {
		t.Fatalf("expected one category, got %+v", report)
	}
	category := report[0]
	if category.GroupName != "Discord" || category.Reason != event.Reason {
		t.Fatalf("unexpected category metadata: %+v", category)
	}
	if len(category.Domains) != 1 || category.Domains[0].Domain != "discord.com" {
		t.Fatalf("unexpected domains: %+v", category.Domains)
	}
	counts := category.Domains[0].Counts
	if counts[PeriodMorning] != 2 || counts[PeriodEvening] != 1 || counts[PeriodAfternoon] != 0 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
}

func TestDigestOnlyIncludesRequestedProfiles(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.Write(gateway.Event{ProfileID: "p1", Action: policy.ActionObserve, MatchedDomain: "discord.com", Timestamp: time.Now()})
	store.Write(gateway.Event{ProfileID: "p2", Action: policy.ActionObserve, MatchedDomain: "roblox.com", Timestamp: time.Now()})

	report := store.Digest([]string{"p1"})
	if len(report) != 1 || len(report[0].Domains) != 1 || report[0].Domains[0].Domain != "discord.com" {
		t.Fatalf("expected only p1's domain, got %+v", report)
	}
}

func TestCompleteDigestResetsOnlyGivenProfilesAndMarksSent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observations.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store.Write(gateway.Event{ProfileID: "p1", Action: policy.ActionObserve, MatchedDomain: "discord.com", Timestamp: time.Now()})
	store.Write(gateway.Event{ProfileID: "p2", Action: policy.ActionObserve, MatchedDomain: "roblox.com", Timestamp: time.Now()})

	now := time.Now()
	if err := store.CompleteDigest("house-1", []string{"p1"}, now); err != nil {
		t.Fatal(err)
	}

	if report := store.Digest([]string{"p1"}); len(report) != 0 {
		t.Fatalf("expected p1 entries cleared, got %+v", report)
	}
	if report := store.Digest([]string{"p2"}); len(report) != 1 {
		t.Fatalf("expected p2 entries untouched, got %+v", report)
	}
	if store.IsDue("house-1", now, time.Hour) {
		t.Fatal("expected house-1 not due right after sending")
	}
	if !store.IsDue("house-2", now, time.Hour) {
		t.Fatal("expected a house that never received a digest to be due")
	}

	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.IsDue("house-1", now, time.Hour) {
		t.Fatal("expected last-sent timestamp to survive reload from disk")
	}
	if report := reloaded.Digest([]string{"p2"}); len(report) != 1 {
		t.Fatalf("expected p2 entries to survive reload, got %+v", report)
	}
}

func TestForgetRemovesEntriesAndLastSentWithoutMarkingSent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observations.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store.Write(gateway.Event{ProfileID: "p1", Action: policy.ActionObserve, MatchedDomain: "discord.com", Timestamp: time.Now()})
	store.Write(gateway.Event{ProfileID: "p2", Action: policy.ActionObserve, MatchedDomain: "roblox.com", Timestamp: time.Now()})
	now := time.Now()
	if err := store.CompleteDigest("house-1", []string{"p1"}, now); err != nil {
		t.Fatal(err)
	}

	if err := store.Forget("house-1", []string{"p1"}); err != nil {
		t.Fatal(err)
	}

	if !store.IsDue("house-1", now, time.Hour) {
		t.Fatal("expected Forget to clear last-sent bookkeeping for the deleted house")
	}
	if report := store.Digest([]string{"p2"}); len(report) != 1 {
		t.Fatalf("expected an unrelated profile to survive Forget, got %+v", report)
	}

	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.IsDue("house-1", now, time.Hour) {
		t.Fatal("expected Forget to persist across reload")
	}
}
