package fixture

import (
	"fmt"
	"strings"

	"github.com/miekg/dns"
)

func Serve(address string) error {
	server := &dns.Server{Addr: address, Net: "udp", Handler: dns.HandlerFunc(handle)}
	return server.ListenAndServe()
}

func handle(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	if len(request.Question) != 1 {
		response.SetRcode(request, dns.RcodeFormatError)
		_ = writer.WriteMsg(response)
		return
	}

	question := request.Question[0]
	name := strings.ToLower(strings.TrimSuffix(question.Name, "."))
	records := map[string]string{
		"allowed.test":        "192.0.2.10",
		"blocked.test":        "192.0.2.20",
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
