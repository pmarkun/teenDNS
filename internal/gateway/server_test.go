package gateway

import (
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/pmarkun/teendns/internal/policy"
)

func TestBlockedResponseHasShortNegativeTTL(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("blocked.test.", dns.TypeA)
	response := blockedResponse(request)

	if response.Rcode != dns.RcodeNameError {
		t.Fatalf("expected NXDOMAIN, got %s", dns.RcodeToString[response.Rcode])
	}
	if len(response.Ns) != 1 || response.Ns[0].Header().Ttl != 30 {
		t.Fatalf("expected SOA with 30 second TTL, got %#v", response.Ns)
	}
}

func TestCapTTLLeavesOPTRecordUntouched(t *testing.T) {
	answer, err := dns.NewRR("shared.test. 600 IN A 192.0.2.30")
	if err != nil {
		t.Fatal(err)
	}
	message := &dns.Msg{Answer: []dns.RR{answer}}
	message.SetEdns0(1232, false)

	capTTL(message, 300)

	if message.Answer[0].Header().Ttl != 300 {
		t.Fatalf("expected capped TTL 300, got %d", message.Answer[0].Header().Ttl)
	}
	if message.IsEdns0() == nil || message.IsEdns0().UDPSize() != 1232 {
		t.Fatal("OPT record was unexpectedly modified")
	}
}

func TestBlockedAliasEvaluatesCNAMETarget(t *testing.T) {
	profile := policy.Profile{
		DefaultAction: policy.ActionAllow,
		Version:       4,
		Rules: []policy.Rule{
			{Domain: "blocked.test", IncludeSubdomains: true, Action: policy.ActionBlock, Category: "demo"},
		},
	}
	record, err := dns.NewRR("alias.test. 600 IN CNAME content.blocked.test.")
	if err != nil {
		t.Fatal(err)
	}
	decision, blocked := blockedAlias(profile, &dns.Msg{Answer: []dns.RR{record}})
	if !blocked || decision.Action != policy.ActionBlock || decision.PolicyVersion != 4 {
		t.Fatalf("expected blocked CNAME target, got %+v, %v", decision, blocked)
	}
}

type recordingSink struct {
	events []Event
}

func (s *recordingSink) Write(event Event) {
	s.events = append(s.events, event)
}

func TestResolveRecordsReasonAndGroupNameOnBlock(t *testing.T) {
	profile := policy.Profile{
		ID:            "home",
		DefaultAction: policy.ActionAllow,
		Version:       7,
		Groups: []policy.RuleGroup{{
			ID:       "gambling",
			Name:     "Apostas",
			Action:   policy.ActionBlock,
			Category: "gambling",
			Reason:   "Apostas usam dinheiro real",
			Domains:  []string{"bet.test"},
		}},
	}
	sink := &recordingSink{}
	server := NewServer("", nil, nil, "", 300, sink, nil)
	request := new(dns.Msg)
	request.SetQuestion("bet.test.", dns.TypeA)

	response := server.resolve(profile, request)
	if response.Rcode != dns.RcodeNameError {
		t.Fatalf("expected NXDOMAIN, got %s", dns.RcodeToString[response.Rcode])
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected exactly one recorded event, got %d", len(sink.events))
	}
	event := sink.events[0]
	if event.Reason != "Apostas usam dinheiro real" || event.MatchedDomain != "bet.test" || event.GroupName != "Apostas" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestResolveBlocksEveryDomainDuringProfilePause(t *testing.T) {
	location, _ := time.LoadLocation("America/Sao_Paulo")
	profile := policy.Profile{
		ID:            "home",
		TimeZone:      "America/Sao_Paulo",
		DefaultAction: policy.ActionAllow,
		Rules:         []policy.Rule{{Domain: "otherwise-allowed.test", Action: policy.ActionAllow}},
		Groups:        []policy.RuleGroup{{ID: "all", Name: "Tudo", Action: policy.ActionAllow, Domains: []string{"otherwise-allowed.test"}, Schedules: []policy.ScheduledAction{{TimeWindow: policy.TimeWindow{ID: "allow", Label: "Sempre", Days: []int{1}, Start: "12:00", End: "13:00"}, Action: policy.ActionAllow}}}},
		Pauses:        []policy.TimeWindow{{ID: "meal", Label: "Refeição", Days: []int{1}, Start: "12:00", End: "13:00"}},
	}
	sink := &recordingSink{}
	server := NewServer("", nil, nil, "", 300, sink, nil)
	server.now = func() time.Time { return time.Date(2026, time.September, 21, 12, 30, 0, 0, location) }
	request := new(dns.Msg)
	request.SetQuestion("otherwise-allowed.test.", dns.TypeA)

	response := server.resolve(profile, request)
	if response.Rcode != dns.RcodeNameError {
		t.Fatalf("expected paused profile to return NXDOMAIN, got %s", dns.RcodeToString[response.Rcode])
	}
	if len(sink.events) != 1 || sink.events[0].Action != policy.ActionBlock || sink.events[0].Category != "global_pause" || sink.events[0].Reason != "Pausa geral" {
		t.Fatalf("expected a pause event, got %+v", sink.events)
	}
}

type pairingStub struct {
	profileID string
	queryName string
}

func (p *pairingStub) Observe(profileID, queryName string) bool {
	p.profileID = profileID
	p.queryName = queryName
	return true
}

func TestPairingChallengeIsObservedBeforePolicyResolution(t *testing.T) {
	observer := &pairingStub{}
	server := NewServer("", nil, nil, "", 300, nil, observer)
	request := new(dns.Msg)
	request.SetQuestion("token.pair.teendns.test.", dns.TypeAAAA)
	response := server.resolve(policy.Profile{ID: "home", DefaultAction: policy.ActionAllow}, request)
	if response.Rcode != dns.RcodeNameError {
		t.Fatalf("expected pairing response to stop resolution, got %s", dns.RcodeToString[response.Rcode])
	}
	if observer.profileID != "home" || observer.queryName != "token.pair.teendns.test." {
		t.Fatalf("unexpected pairing observation: %+v", observer)
	}
}
