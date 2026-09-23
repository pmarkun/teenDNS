package fixture

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type handler struct {
	mu     sync.Mutex
	counts map[string]int
}

func Serve(address, metricsAddress string) error {
	h := &handler{counts: make(map[string]int)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /count", h.count)
	metrics := &http.Server{Addr: metricsAddress, Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	go func() {
		if err := metrics.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("fixture metrics: %v", err)
		}
	}()

	server := &dns.Server{Addr: address, Net: "udp", Handler: h}
	return server.ListenAndServe()
}

func (h *handler) ServeDNS(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	if len(request.Question) != 1 {
		response.SetRcode(request, dns.RcodeFormatError)
		_ = writer.WriteMsg(response)
		return
	}

	question := request.Question[0]
	name := strings.ToLower(strings.TrimSuffix(question.Name, "."))
	h.mu.Lock()
	h.counts[name]++
	h.mu.Unlock()
	if name == "alias.test" && question.Qtype == dns.TypeA {
		record, err := dns.NewRR("alias.test. 600 IN CNAME blocked.test.")
		if err == nil {
			response.Answer = []dns.RR{record}
		}
		_ = writer.WriteMsg(response)
		return
	}
	records := map[string]string{
		"allowed.test":        "192.0.2.10",
		"blocked.test":        "192.0.2.20",
		"dynamic.test":        "192.0.2.50",
		"shared.test":         "192.0.2.30",
		"school.blocked.test": "192.0.2.40",
	}
	ip, found := records[name]
	if !found {
		response.SetRcode(request, dns.RcodeNameError)
		_ = writer.WriteMsg(response)
		return
	}
	if question.Qtype == dns.TypeA {
		record, err := dns.NewRR(fmt.Sprintf("%s 600 IN A %s", question.Name, ip))
		if err == nil {
			response.Answer = []dns.RR{record}
		}
	}
	_ = writer.WriteMsg(response)
}

func (h *handler) count(writer http.ResponseWriter, request *http.Request) {
	name := strings.ToLower(strings.TrimSuffix(request.URL.Query().Get("name"), "."))
	h.mu.Lock()
	count := h.counts[name]
	h.mu.Unlock()
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = writer.Write([]byte(strconv.Itoa(count)))
}
