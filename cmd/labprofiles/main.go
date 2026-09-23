package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/policy"
)

func main() {
	path := flag.String("config", ".local/gateway.json", "laboratory configuration")
	count := flag.Int("count", 50, "number of synthetic profiles")
	flag.Parse()

	if *count < 1 || *count > 10000 {
		log.Fatal("count must be between 1 and 10000")
	}
	cfg, err := config.Load(*path)
	if err != nil {
		log.Fatal(err)
	}

	profiles := make([]policy.Profile, 0, len(cfg.Profiles)+*count)
	for _, profile := range cfg.Profiles {
		if len(profile.ID) < 5 || profile.ID[:5] != "load-" {
			profiles = append(profiles, profile)
		}
	}
	for index := 1; index <= *count; index++ {
		profiles = append(profiles, policy.Profile{
			ID:            fmt.Sprintf("load-%03d", index),
			Hostname:      fmt.Sprintf("p-load-%03d.dns.teendns.test", index),
			DefaultAction: policy.ActionAllow,
			Version:       1,
		})
	}
	cfg.Profiles = profiles
	if _, err := policy.NewStore(cfg.Profiles); err != nil {
		log.Fatal(err)
	}
	if err := config.WriteAtomic(*path, cfg, 0o644); err != nil {
		log.Fatal(err)
	}
}
