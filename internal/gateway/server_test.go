package gateway

import (
	"testing"

	"github.com/miekg/dns"
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
