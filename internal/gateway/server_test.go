package gateway

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
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

func TestDoHHandlerSupportsGetAndPostWithProfilePolicy(t *testing.T) {
	manager, err := policy.NewManager([]policy.Profile{{
		ID: "home", Hostname: "p-secret.dns.test", DefaultAction: policy.ActionAllow,
		Groups: []policy.RuleGroup{{ID: "blocked", Name: "Bloqueado", Action: policy.ActionBlock, Domains: []string{"blocked.test"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	sink := &recordingSink{}
	server := NewServer("", nil, manager, "", 300, sink, nil)
	requestMessage := new(dns.Msg)
	requestMessage.SetQuestion("blocked.test.", dns.TypeA)
	wire, err := requestMessage.Pack()
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "/dns-query/p-secret"
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, endpoint+"?dns="+base64.RawURLEncoding.EncodeToString(wire), nil),
		httptest.NewRequest(http.MethodPost, endpoint, bytes.NewReader(wire)),
	}
	requests[1].Header.Set("Content-Type", "application/dns-message")

	for _, request := range requests {
		response := httptest.NewRecorder()
		server.DoHHandler().ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/dns-message" {
			t.Fatalf("unexpected DoH response: %d %q %s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
		}
		var dnsResponse dns.Msg
		if err := dnsResponse.Unpack(response.Body.Bytes()); err != nil {
			t.Fatalf("response is not a DNS message: %v", err)
		}
		if dnsResponse.Rcode != dns.RcodeNameError {
			t.Fatalf("expected the profile rule to return NXDOMAIN, got %s", dns.RcodeToString[dnsResponse.Rcode])
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("DoH response is cacheable: %q", response.Header().Get("Cache-Control"))
		}
	}
	if len(sink.events) != 2 || sink.events[0].ProfileID != "home" || sink.events[1].ProfileID != "home" {
		t.Fatalf("DoH requests were not attributed to their profile: %+v", sink.events)
	}
}

func TestDoHHandlerRejectsMalformedAndUnknownRequests(t *testing.T) {
	manager, err := policy.NewManager([]policy.Profile{{ID: "home", Hostname: "p-secret.dns.test", DefaultAction: policy.ActionAllow}})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewServer("", nil, manager, "", 300, nil, nil).DoHHandler()
	cases := []struct {
		name   string
		method string
		path   string
		typeOf string
		body   string
		status int
	}{
		{name: "unknown profile", method: http.MethodGet, path: "/dns-query/p-other?dns=AA", status: http.StatusNotFound},
		{name: "missing dns parameter", method: http.MethodGet, path: "/dns-query/p-secret", status: http.StatusBadRequest},
		{name: "malformed dns parameter", method: http.MethodGet, path: "/dns-query/p-secret?dns=not-base64", status: http.StatusBadRequest},
		{name: "unsupported media type", method: http.MethodPost, path: "/dns-query/p-secret", typeOf: "application/json", body: "{}", status: http.StatusUnsupportedMediaType},
		{name: "invalid dns packet", method: http.MethodPost, path: "/dns-query/p-secret", typeOf: "application/dns-message", body: "bad", status: http.StatusBadRequest},
		{name: "unsupported method", method: http.MethodPut, path: "/dns-query/p-secret", status: http.StatusMethodNotAllowed},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			if test.typeOf != "" {
				request.Header.Set("Content-Type", test.typeOf)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("expected HTTP %d, got %d: %s", test.status, response.Code, response.Body.String())
			}
		})
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
