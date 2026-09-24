package gateway

import (
	"testing"

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
