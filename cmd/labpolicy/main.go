package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/policy"
)

func main() {
	path := flag.String("config", ".local/gateway.json", "laboratory configuration")
	hostname := flag.String("hostname", "", "profile hostname")
	domain := flag.String("domain", "", "rule domain")
	action := flag.String("action", "", "allow, block or observe")
	flag.Parse()

	if *hostname == "" || *domain == "" || *action == "" {
		log.Fatal("hostname, domain and action are required")
	}
	cfg, err := config.Load(*path)
	if err != nil {
		log.Fatal(err)
	}

	found := false
	for profileIndex := range cfg.Profiles {
		profile := &cfg.Profiles[profileIndex]
		if profile.Hostname != *hostname {
			continue
		}
		found = true
		profile.Version++
		updated := false
		for ruleIndex := range profile.Rules {
			if profile.Rules[ruleIndex].Domain == *domain {
				profile.Rules[ruleIndex].Action = policy.Action(*action)
				updated = true
			}
		}
		if !updated {
			profile.Rules = append(profile.Rules, policy.Rule{Domain: *domain, Action: policy.Action(*action)})
		}
	}
	if !found {
		log.Fatalf("profile %q not found", *hostname)
	}
	if _, err := policy.NewStore(cfg.Profiles); err != nil {
		log.Fatal(err)
	}

	contents, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	contents = append(contents, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(*path), "gateway-*.json")
	if err != nil {
		log.Fatal(err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(contents); err != nil {
		log.Fatal(err)
	}
	if err := temporary.Close(); err != nil {
		log.Fatal(err)
	}
	if err := os.Chmod(temporaryPath, 0o644); err != nil {
		log.Fatal(err)
	}
	if err := os.Rename(temporaryPath, *path); err != nil {
		log.Fatal(err)
	}
}
