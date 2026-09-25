package policy

import (
	"testing"
	"time"
)

func TestDecideUsesMostSpecificRule(t *testing.T) {
	profile := Profile{
		DefaultAction: ActionAllow,
		Version:       3,
		Rules: []Rule{
			{Domain: "example.test", IncludeSubdomains: true, Action: ActionBlock},
			{Domain: "school.example.test", Action: ActionAllow},
		},
	}

	decision, err := Decide(profile, "school.example.test.")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionAllow {
		t.Fatalf("expected allow, got %q", decision.Action)
	}
	if decision.PolicyVersion != 3 {
		t.Fatalf("expected version 3, got %d", decision.PolicyVersion)
	}
}

func TestDecideMatchesDomainInsideRuleGroup(t *testing.T) {
	profile := Profile{
		DefaultAction: ActionAllow,
		Version:       3,
		Groups: []RuleGroup{{
			ID:       "gambling",
			Name:     "Apostas",
			Action:   ActionBlock,
			Category: "gambling",
			Domains:  []string{"example.bet.br"},
		}},
	}
	store, err := NewStore([]Profile{{ID: "home", Hostname: "home.test", DefaultAction: ActionAllow, Groups: profile.Groups}})
	if err != nil || store == nil {
		t.Fatalf("group validation failed: %v", err)
	}
	decision, err := Decide(profile, "promo.example.bet.br")
	if err != nil || decision.Action != ActionBlock || decision.Category != "gambling" {
		t.Fatalf("unexpected decision: %+v, %v", decision, err)
	}
	if decision.GroupName != "Apostas" {
		t.Fatalf("expected group name %q, got %q", "Apostas", decision.GroupName)
	}
}

func TestDecideLeavesGroupNameEmptyForRuleMatch(t *testing.T) {
	profile := Profile{
		DefaultAction: ActionAllow,
		Rules: []Rule{
			{Domain: "blocked.test", Action: ActionBlock, Category: "demo"},
		},
	}

	decision, err := Decide(profile, "blocked.test")
	if err != nil {
		t.Fatal(err)
	}
	if decision.GroupName != "" {
		t.Fatalf("expected empty group name for rule match, got %q", decision.GroupName)
	}
}

func TestDecideDoesNotMatchDeceptiveSuffix(t *testing.T) {
	profile := Profile{
		DefaultAction: ActionAllow,
		Rules: []Rule{
			{Domain: "google.com", IncludeSubdomains: true, Action: ActionBlock},
		},
	}

	decision, err := Decide(profile, "google.com.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionAllow {
		t.Fatalf("expected deceptive suffix to remain allowed, got %q", decision.Action)
	}
}

func TestDecideNormalizesCaseAndTrailingDot(t *testing.T) {
	profile := Profile{
		DefaultAction: ActionAllow,
		Rules: []Rule{
			{Domain: "blocked.test", Action: ActionBlock, Category: "demo"},
		},
	}

	decision, err := Decide(profile, "BLOCKED.TEST.")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionBlock || decision.Category != "demo" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestStoreRejectsDuplicateHostname(t *testing.T) {
	_, err := NewStore([]Profile{
		{ID: "one", Hostname: "p-one.dns.teendns.test", DefaultAction: ActionAllow},
		{ID: "two", Hostname: "P-ONE.DNS.TEENDNS.TEST.", DefaultAction: ActionAllow},
	})
	if err == nil {
		t.Fatal("expected duplicate hostname error")
	}
}

func TestStoreFindsNormalizedHostname(t *testing.T) {
	store, err := NewStore([]Profile{
		{ID: "ana", Hostname: "p-ana.dns.teendns.test", DefaultAction: ActionAllow},
	})
	if err != nil {
		t.Fatal(err)
	}
	profile, ok := store.Profile("P-ANA.DNS.TEENDNS.TEST.")
	if !ok || profile.ID != "ana" {
		t.Fatalf("unexpected profile lookup: %+v, %v", profile, ok)
	}
}

func TestManagerFindsProfileByOpaqueHostnameLabel(t *testing.T) {
	manager, err := NewManager([]Profile{{
		ID: "ana", Hostname: "p-secret.dns.teendns.test", DefaultAction: ActionAllow,
	}})
	if err != nil {
		t.Fatal(err)
	}
	profile, ok := manager.ProfileLabel("P-SECRET")
	if !ok || profile.ID != "ana" {
		t.Fatalf("unexpected opaque-label lookup: %+v, %v", profile, ok)
	}
	if _, ok := manager.ProfileLabel("p-secret.other.dns.test"); ok {
		t.Fatal("profile label lookup must accept a single opaque label, not a hostname")
	}
}

func TestStoreRejectsDuplicateOpaqueProfileLabels(t *testing.T) {
	_, err := NewStore([]Profile{
		{ID: "one", Hostname: "p-shared.dns-one.test", DefaultAction: ActionAllow},
		{ID: "two", Hostname: "p-shared.dns-two.test", DefaultAction: ActionAllow},
	})
	if err == nil {
		t.Fatal("expected duplicate opaque labels to be rejected")
	}
}

func TestManagerReplacesProfilesAtomically(t *testing.T) {
	manager, err := NewManager([]Profile{
		{ID: "ana", Hostname: "p-ana.dns.teendns.test", DefaultAction: ActionAllow, Version: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Replace([]Profile{
		{ID: "ana", Hostname: "p-ana.dns.teendns.test", DefaultAction: ActionBlock, Version: 2},
	}); err != nil {
		t.Fatal(err)
	}
	profile, ok := manager.Profile("p-ana.dns.teendns.test")
	if !ok || profile.Version != 2 || profile.DefaultAction != ActionBlock {
		t.Fatalf("unexpected profile after replacement: %+v, %v", profile, ok)
	}
}

func TestDecideAtAppliesScheduledGroupActionOnlyDuringWindow(t *testing.T) {
	profile := Profile{
		TimeZone:      "America/Sao_Paulo",
		DefaultAction: ActionAllow,
		Groups: []RuleGroup{{
			ID: "games", Name: "Jogos", Action: ActionBlock, Domains: []string{"games.test"},
			Schedules: []ScheduledAction{{TimeWindow: TimeWindow{ID: "after-school", Label: "Depois da escola", Days: []int{1}, Start: "16:00", End: "17:00"}, Action: ActionAllow}},
		}},
	}
	location, _ := time.LoadLocation("America/Sao_Paulo")
	cases := []struct {
		name string
		time time.Time
		want Action
	}{
		{name: "inside", time: time.Date(2026, time.September, 21, 16, 0, 0, 0, location), want: ActionAllow},
		{name: "end is exclusive", time: time.Date(2026, time.September, 21, 17, 0, 0, 0, location), want: ActionBlock},
		{name: "other weekday", time: time.Date(2026, time.September, 22, 16, 30, 0, 0, location), want: ActionBlock},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			decision, err := DecideAt(profile, "games.test", test.time)
			if err != nil || decision.Action != test.want {
				t.Fatalf("unexpected decision: %+v, %v", decision, err)
			}
		})
	}
	blockSchedule := Profile{
		TimeZone:      "America/Sao_Paulo",
		DefaultAction: ActionAllow,
		Groups: []RuleGroup{{
			ID: "games", Name: "Jogos", Action: ActionAllow, Domains: []string{"games.test"},
			Schedules: []ScheduledAction{{TimeWindow: TimeWindow{ID: "study", Label: "Estudos", Days: []int{2}, Start: "16:00", End: "17:00"}, Action: ActionBlock}},
		}},
	}
	blocked, err := DecideAt(blockSchedule, "games.test", time.Date(2026, time.September, 22, 16, 30, 0, 0, location))
	if err != nil || blocked.Action != ActionBlock || blocked.ScheduleLabel != "Estudos" {
		t.Fatalf("scheduled block was not applied: %+v, %v", blocked, err)
	}
}

func TestDecideAtSupportsOvernightPauseAndPauseOverridesRules(t *testing.T) {
	profile := Profile{
		TimeZone:      "America/Sao_Paulo",
		DefaultAction: ActionAllow,
		Rules:         []Rule{{Domain: "games.test", Action: ActionAllow}},
		Pauses:        []TimeWindow{{ID: "sleep", Label: "Dormir", Days: []int{1}, Start: "22:00", End: "07:00"}},
	}
	location, _ := time.LoadLocation("America/Sao_Paulo")
	cases := []struct {
		name string
		time time.Time
		want Action
	}{
		{name: "start day late night", time: time.Date(2026, time.September, 21, 22, 0, 0, 0, location), want: ActionBlock},
		{name: "following morning", time: time.Date(2026, time.September, 22, 6, 59, 0, 0, location), want: ActionBlock},
		{name: "end is exclusive", time: time.Date(2026, time.September, 22, 7, 0, 0, 0, location), want: ActionAllow},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			decision, err := DecideAt(profile, "games.test", test.time)
			if err != nil || decision.Action != test.want {
				t.Fatalf("unexpected decision: %+v, %v", decision, err)
			}
			if test.want == ActionBlock && (decision.Category != "global_pause" || decision.ScheduleLabel != "Dormir") {
				t.Fatalf("global pause not identified in decision: %+v", decision)
			}
		})
	}
}

func TestStoreRejectsOverlappingGroupSchedules(t *testing.T) {
	_, err := NewStore([]Profile{{
		ID: "ana", Hostname: "ana.test", DefaultAction: ActionAllow,
		Groups: []RuleGroup{{
			ID: "games", Name: "Jogos", Action: ActionBlock, Domains: []string{"games.test"},
			Schedules: []ScheduledAction{
				{TimeWindow: TimeWindow{ID: "sleep", Label: "Dormir", Days: []int{1}, Start: "22:00", End: "06:00"}, Action: ActionBlock},
				{TimeWindow: TimeWindow{ID: "morning", Label: "Manhã", Days: []int{2}, Start: "05:00", End: "08:00"}, Action: ActionAllow},
			},
		}},
	}})
	if err == nil {
		t.Fatal("expected overlapping schedules to be rejected")
	}
}

func TestStoreRejectsInvalidTimeWindows(t *testing.T) {
	for _, window := range []TimeWindow{
		{ID: "empty-days", Label: "Dormir", Start: "22:00", End: "07:00"},
		{ID: "bad-time", Label: "Dormir", Days: []int{1}, Start: "25:00", End: "07:00"},
		{ID: "zero-duration", Label: "Dormir", Days: []int{1}, Start: "07:00", End: "07:00"},
		{ID: "bad-day", Label: "Dormir", Days: []int{7}, Start: "22:00", End: "07:00"},
	} {
		_, err := NewStore([]Profile{{
			ID: "ana", Hostname: "ana.test", DefaultAction: ActionAllow,
			Pauses: []TimeWindow{window},
		}})
		if err == nil {
			t.Errorf("expected invalid window to fail validation: %+v", window)
		}
	}
}

func TestDecideAtRejectsInvalidTimeZoneWhenCalledWithoutStore(t *testing.T) {
	_, err := DecideAt(Profile{TimeZone: "Mars/Olympus", DefaultAction: ActionAllow}, "example.test", time.Now())
	if err == nil {
		t.Fatal("expected an invalid profile time zone to return an error")
	}
}
