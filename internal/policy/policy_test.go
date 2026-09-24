package policy

import "testing"

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
