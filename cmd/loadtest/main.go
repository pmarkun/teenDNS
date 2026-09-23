package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

type result struct {
	duration time.Duration
	err      error
}

func main() {
	address := flag.String("address", "127.0.0.1:8853", "DoT gateway address")
	caPath := flag.String("ca", ".local/certs/ca.pem", "CA certificate")
	profiles := flag.Int("profiles", 50, "number of synthetic profiles")
	requests := flag.Int("requests", 500, "total requests in burst mode")
	concurrency := flag.Int("concurrency", 100, "concurrent workers")
	duration := flag.Duration("duration", 0, "sustained test duration")
	rate := flag.Int("rate", 50, "requests per second in sustained mode")
	minimumQPS := flag.Float64("minimum-qps", 50, "minimum throughput in burst mode")
	flag.Parse()

	if *profiles < 1 || *concurrency < 1 || *requests < 1 || *rate < 1 {
		log.Fatal("profiles, concurrency, requests and rate must be positive")
	}
	roots := loadRoots(*caPath)

	tasks := make(chan int)
	results := make(chan result, *concurrency)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for worker := 0; worker < *concurrency; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for index := range tasks {
				serverName := fmt.Sprintf("p-load-%03d.dns.teendns.test", (index%*profiles)+1)
				started := time.Now()
				err := query(*address, serverName, roots)
				results <- result{duration: time.Since(started), err: err}
			}
		}()
	}

	testStarted := time.Now()
	close(start)
	var sent atomic.Int64
	go func() {
		defer close(tasks)
		if *duration == 0 {
			for index := 0; index < *requests; index++ {
				tasks <- index
				sent.Add(1)
			}
			return
		}
		interval := time.Second / time.Duration(*rate)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		deadline := time.NewTimer(*duration)
		defer deadline.Stop()
		for index := 0; ; index++ {
			select {
			case <-deadline.C:
				return
			case <-ticker.C:
				tasks <- index
				sent.Add(1)
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	latencies := make([]time.Duration, 0, *requests)
	failures := 0
	for outcome := range results {
		latencies = append(latencies, outcome.duration)
		if outcome.err != nil {
			failures++
			if failures <= 5 {
				log.Printf("query failed: %v", outcome.err)
			}
		}
	}
	elapsed := time.Since(testStarted)
	if failures > 0 {
		log.Fatalf("%d of %d queries failed", failures, len(latencies))
	}
	if len(latencies) == 0 {
		log.Fatal("no queries completed")
	}
	sort.Slice(latencies, func(left, right int) bool { return latencies[left] < latencies[right] })
	p95 := latencies[(len(latencies)*95-1)/100]
	throughput := float64(len(latencies)) / elapsed.Seconds()
	fmt.Printf("ok profiles=%d queries=%d elapsed=%s qps=%.1f p95=%s\n", *profiles, sent.Load(), elapsed.Round(time.Millisecond), throughput, p95.Round(time.Microsecond))
	if *duration == 0 && throughput < *minimumQPS {
		log.Fatalf("throughput %.1f qps is below minimum %.1f", throughput, *minimumQPS)
	}
}

func loadRoots(path string) *x509.CertPool {
	contents, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(contents) {
		log.Fatal("could not parse CA certificate")
	}
	return roots
}

func query(address, serverName string, roots *x509.CertPool) error {
	message := new(dns.Msg)
	message.SetQuestion("shared.test.", dns.TypeA)
	client := &dns.Client{
		Net:     "tcp-tls",
		Timeout: 5 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
			RootCAs:    roots,
			ServerName: serverName,
		},
	}
	response, _, err := client.Exchange(message, address)
	if err != nil {
		return err
	}
	if response.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("profile %s returned %s", serverName, dns.RcodeToString[response.Rcode])
	}
	return nil
}
