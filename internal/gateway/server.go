package gateway

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/pmarkun/teendns/internal/policy"
)

type Event struct {
	Timestamp     time.Time     `json:"timestamp"`
	ProfileID     string        `json:"profile_id"`
	Query         string        `json:"query"`
	QueryType     uint16        `json:"query_type"`
	Action        policy.Action `json:"action"`
	Category      string        `json:"category,omitempty"`
	Reason        string        `json:"reason,omitempty"`
	MatchedDomain string        `json:"matched_domain,omitempty"`
	GroupName     string        `json:"group_name,omitempty"`
	PolicyVersion int64         `json:"policy_version"`
}

type EventWriter struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

type EventSink interface {
	Write(Event)
}

type PairingObserver interface {
	Observe(profileID, queryName string) bool
}

func NewEventWriter(writer io.Writer) *EventWriter {
	return &EventWriter{encoder: json.NewEncoder(writer)}
}

func (w *EventWriter) Write(event Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.encoder.Encode(event); err != nil {
		log.Printf("encode event: %v", err)
	}
}

type Server struct {
	address  string
	tls      *tls.Config
	profiles ProfileLookup
	upstream string
	maxTTL   uint32
	events   EventSink
	pairings PairingObserver
	now      func() time.Time
}

type ProfileLookup interface {
	Profile(hostname string) (policy.Profile, bool)
}

func NewServer(address string, tlsConfig *tls.Config, profiles ProfileLookup, upstream string, maxTTL uint32, events EventSink, pairings PairingObserver) *Server {
	return &Server{
		address:  address,
		tls:      tlsConfig,
		profiles: profiles,
		upstream: upstream,
		maxTTL:   maxTTL,
		events:   events,
		pairings: pairings,
		now:      time.Now,
	}
}

func (s *Server) Serve(ctx context.Context) error {
	listener, err := tls.Listen("tcp", s.address, s.tls)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Printf("accept connection: %v", err)
			continue
		}
		go s.handleConnection(ctx, connection.(*tls.Conn))
	}
}

// DoHHandler serves RFC 8484 GET and POST requests. The opaque profile label
// in the path selects the same policy used by DNS-over-TLS; reverse proxies
// should disable access logging for this path because it contains that token.
func (s *Server) DoHHandler() http.Handler {
	return http.HandlerFunc(s.handleDoH)
}

func (s *Server) handleDoH(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		writer.Header().Set("Allow", "GET, POST")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	profileLabel := strings.TrimPrefix(request.URL.Path, "/dns-query/")
	if profileLabel == request.URL.Path || profileLabel == "" || strings.Contains(profileLabel, "/") {
		http.NotFound(writer, request)
		return
	}
	profileLookup, ok := s.profiles.(interface {
		ProfileLabel(string) (policy.Profile, bool)
	})
	if !ok {
		http.NotFound(writer, request)
		return
	}
	profile, ok := profileLookup.ProfileLabel(profileLabel)
	if !ok {
		http.NotFound(writer, request)
		return
	}

	messageBytes, status := dohRequestMessage(writer, request)
	if status != 0 {
		return
	}
	dnsRequest := new(dns.Msg)
	if err := dnsRequest.Unpack(messageBytes); err != nil {
		http.Error(writer, "invalid DNS message", http.StatusBadRequest)
		return
	}

	dnsResponse := s.resolve(profile, dnsRequest)
	responseBytes, err := dnsResponse.Pack()
	if err != nil {
		http.Error(writer, "could not encode DNS response", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/dns-message")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(responseBytes)
}

func dohRequestMessage(writer http.ResponseWriter, request *http.Request) ([]byte, int) {
	if request.Method == http.MethodGet {
		encoded := request.URL.Query().Get("dns")
		if encoded == "" {
			http.Error(writer, "missing dns parameter", http.StatusBadRequest)
			return nil, http.StatusBadRequest
		}
		message, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			message, err = base64.URLEncoding.DecodeString(encoded)
		}
		if err != nil {
			http.Error(writer, "invalid dns parameter", http.StatusBadRequest)
			return nil, http.StatusBadRequest
		}
		if len(message) > 65535 {
			http.Error(writer, "DNS message is too large", http.StatusRequestEntityTooLarge)
			return nil, http.StatusRequestEntityTooLarge
		}
		return message, 0
	}

	contentType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || contentType != "application/dns-message" {
		http.Error(writer, "Content-Type must be application/dns-message", http.StatusUnsupportedMediaType)
		return nil, http.StatusUnsupportedMediaType
	}
	message, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 65535))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(writer, "DNS message is too large", http.StatusRequestEntityTooLarge)
			return nil, http.StatusRequestEntityTooLarge
		}
		http.Error(writer, "could not read DNS message", http.StatusBadRequest)
		return nil, http.StatusBadRequest
	}
	return message, 0
}

func (s *Server) handleConnection(ctx context.Context, connection *tls.Conn) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(30 * time.Second))
	if err := connection.HandshakeContext(ctx); err != nil {
		log.Printf("TLS handshake: %v", err)
		return
	}

	serverName := connection.ConnectionState().ServerName
	profile, ok := s.profiles.Profile(serverName)
	if !ok {
		log.Printf("unknown profile endpoint %q", serverName)
		return
	}

	dnsConnection := &dns.Conn{Conn: connection}
	for {
		request, err := dnsConnection.ReadMsg()
		if err != nil {
			if !errors.Is(err, io.EOF) && !isTimeout(err) {
				log.Printf("read DNS message for profile %q: %v", profile.ID, err)
			}
			return
		}
		profile, ok = s.profiles.Profile(serverName)
		if !ok {
			log.Printf("profile endpoint %q was revoked during a DNS connection", serverName)
			return
		}
		response := s.resolve(profile, request)
		if err := dnsConnection.WriteMsg(response); err != nil {
			log.Printf("write DNS message for profile %q: %v", profile.ID, err)
			return
		}
		_ = connection.SetDeadline(time.Now().Add(30 * time.Second))
	}
}

func (s *Server) resolve(profile policy.Profile, request *dns.Msg) *dns.Msg {
	if len(request.Question) != 1 {
		response := new(dns.Msg)
		response.SetRcode(request, dns.RcodeFormatError)
		return response
	}

	question := request.Question[0]
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	if s.pairings != nil && s.pairings.Observe(profile.ID, question.Name) {
		return blockedResponse(request)
	}
	decision, err := policy.DecideAt(profile, question.Name, now)
	if err != nil {
		response := new(dns.Msg)
		response.SetRcode(request, dns.RcodeFormatError)
		return response
	}

	event := Event{
		Timestamp:     now.UTC(),
		ProfileID:     profile.ID,
		Query:         strings.ToLower(strings.TrimSuffix(question.Name, ".")),
		QueryType:     question.Qtype,
		Action:        decision.Action,
		Category:      decision.Category,
		Reason:        decision.Reason,
		MatchedDomain: decision.MatchedDomain,
		GroupName:     decision.GroupName,
		PolicyVersion: decision.PolicyVersion,
	}

	if decision.Action == policy.ActionBlock {
		response := blockedResponse(request)
		s.events.Write(event)
		return response
	}

	client := &dns.Client{Net: "udp", Timeout: 3 * time.Second}
	response, _, err := client.Exchange(request, s.upstream)
	if err != nil {
		response = new(dns.Msg)
		response.SetRcode(request, dns.RcodeServerFailure)
	} else {
		if aliasDecision, blocked := blockedAlias(profile, response); blocked {
			event.Action = aliasDecision.Action
			event.Category = aliasDecision.Category
			event.PolicyVersion = aliasDecision.PolicyVersion
			s.events.Write(event)
			return blockedResponse(request)
		}
		capTTL(response, s.maxTTL)
	}
	s.events.Write(event)
	return response
}

func blockedAlias(profile policy.Profile, message *dns.Msg) (policy.Decision, bool) {
	for _, record := range message.Answer {
		alias, ok := record.(*dns.CNAME)
		if !ok {
			continue
		}
		decision, err := policy.Decide(profile, alias.Target)
		if err == nil && decision.Action == policy.ActionBlock {
			return decision, true
		}
	}
	return policy.Decision{}, false
}

func blockedResponse(request *dns.Msg) *dns.Msg {
	response := new(dns.Msg)
	response.SetRcode(request, dns.RcodeNameError)
	response.Ns = []dns.RR{&dns.SOA{
		Hdr: dns.RR_Header{
			Name:   request.Question[0].Name,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    30,
		},
		Ns:      "blocked.teendns.invalid.",
		Mbox:    "hostmaster.teendns.invalid.",
		Serial:  1,
		Refresh: 30,
		Retry:   30,
		Expire:  30,
		Minttl:  30,
	}}
	return response
}

func capTTL(message *dns.Msg, maximum uint32) {
	for _, section := range [][]dns.RR{message.Answer, message.Ns, message.Extra} {
		for _, record := range section {
			if record.Header().Rrtype != dns.TypeOPT && record.Header().Ttl > maximum {
				record.Header().Ttl = maximum
			}
		}
	}
}

func isTimeout(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}
