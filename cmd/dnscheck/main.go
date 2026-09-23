package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/miekg/dns"
)

func main() {
	address := flag.String("address", "127.0.0.1:8853", "DoT gateway address")
	serverName := flag.String("server-name", "", "TLS server name and profile endpoint")
	caPath := flag.String("ca", ".local/certs/ca.pem", "CA certificate")
	query := flag.String("query", "allowed.test", "DNS name")
	wantRcode := flag.String("want-rcode", "NOERROR", "expected response code")
	wantIP := flag.String("want-ip", "", "expected IPv4 address")
	flag.Parse()

	if *serverName == "" {
		log.Fatal("server-name is required")
	}
	caContents, err := os.ReadFile(*caPath)
	if err != nil {
		log.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caContents) {
		log.Fatal("could not parse CA certificate")
	}

	message := new(dns.Msg)
	message.SetQuestion(dns.Fqdn(*query), dns.TypeA)
	client := &dns.Client{
		Net:     "tcp-tls",
		Timeout: 5 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
			RootCAs:    roots,
			ServerName: *serverName,
		},
	}
	response, _, err := client.Exchange(message, *address)
	if err != nil {
		log.Fatal(err)
	}
	gotRcode := dns.RcodeToString[response.Rcode]
	if gotRcode != *wantRcode {
		log.Fatalf("expected rcode %s, got %s", *wantRcode, gotRcode)
	}
	if *wantIP != "" {
		found := false
		for _, answer := range response.Answer {
			if record, ok := answer.(*dns.A); ok && record.A.String() == *wantIP {
				found = true
			}
		}
		if !found {
			log.Fatalf("expected IP %s, got %v", *wantIP, response.Answer)
		}
	}
	fmt.Printf("ok profile=%s query=%s rcode=%s answers=%v\n", *serverName, *query, gotRcode, response.Answer)
}
