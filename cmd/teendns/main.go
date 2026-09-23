package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/fixture"
	"github.com/pmarkun/teendns/internal/gateway"
	"github.com/pmarkun/teendns/internal/policy"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: teendns gateway|fixture")
	}

	switch os.Args[1] {
	case "gateway":
		runGateway(os.Args[2:])
	case "fixture":
		runFixture(os.Args[2:])
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}

func runGateway(arguments []string) {
	flags := flag.NewFlagSet("gateway", flag.ExitOnError)
	configPath := flags.String("config", "config.json", "configuration file")
	_ = flags.Parse(arguments)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	certificate, err := tls.LoadX509KeyPair(cfg.Certificate, cfg.PrivateKey)
	if err != nil {
		log.Fatalf("load TLS certificate: %v", err)
	}
	profiles, err := policy.NewManager(cfg.Profiles)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	reload := make(chan os.Signal, 1)
	signal.Notify(reload, syscall.SIGHUP)
	defer signal.Stop(reload)
	go func() {
		for range reload {
			updated, err := config.Load(*configPath)
			if err != nil {
				log.Printf("reload config: %v", err)
				continue
			}
			if err := profiles.Replace(updated.Profiles); err != nil {
				log.Printf("reload profiles: %v", err)
				continue
			}
			log.Printf("reloaded %d profiles", len(updated.Profiles))
		}
	}()
	server := gateway.NewServer(
		cfg.Listen,
		&tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13},
		profiles,
		cfg.Upstream,
		cfg.MaxTTL,
		gateway.NewEventWriter(os.Stdout),
	)
	log.Printf("teenDNS gateway listening on %s", cfg.Listen)
	if err := server.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}

func runFixture(arguments []string) {
	flags := flag.NewFlagSet("fixture", flag.ExitOnError)
	listen := flags.String("listen", ":5353", "UDP listen address")
	metrics := flags.String("metrics", ":8080", "HTTP metrics listen address")
	_ = flags.Parse(arguments)
	log.Printf("fixture DNS listening on %s", *listen)
	if err := fixture.Serve(*listen, *metrics); err != nil {
		log.Fatal(fmt.Errorf("serve fixture DNS: %w", err))
	}
}
