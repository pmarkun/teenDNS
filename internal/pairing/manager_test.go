package pairing

import (
	"strings"
	"testing"
	"time"
)

func TestChallengePairsDNSProfileWithoutExposingChallengeID(t *testing.T) {
	manager := NewManager("pair.teendns.test", time.Minute, time.Hour)
	challenge, err := manager.Create()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(challenge.DNSName, challenge.ID) {
		t.Fatal("DNS name must not expose the status challenge ID")
	}
	if !manager.Observe("home", challenge.DNSName+".") {
		t.Fatal("active DNS challenge was not observed")
	}
	status, ok := manager.Status(challenge.ID)
	if !ok || !status.Paired || status.SessionToken == "" {
		t.Fatalf("unexpected challenge status: %+v, %v", status, ok)
	}
	profileID, ok := manager.Profile(status.SessionToken)
	if !ok || profileID != "home" {
		t.Fatalf("unexpected session profile: %q, %v", profileID, ok)
	}
}

func TestChallengeRejectsUnrelatedAndNestedNames(t *testing.T) {
	manager := NewManager("pair.teendns.test", time.Minute, time.Hour)
	challenge, _ := manager.Create()
	if manager.Observe("home", "unrelated.test") {
		t.Fatal("unrelated query was accepted")
	}
	if manager.Observe("home", "extra."+challenge.DNSName) {
		t.Fatal("nested challenge name was accepted")
	}
}

func TestChallengeAndSessionExpire(t *testing.T) {
	manager := NewManager("pair.teendns.test", time.Minute, time.Hour)
	now := time.Date(2026, 9, 23, 20, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	challenge, _ := manager.Create()
	now = now.Add(2 * time.Minute)
	if manager.Observe("home", challenge.DNSName) {
		t.Fatal("expired challenge was accepted")
	}
	if _, ok := manager.Status(challenge.ID); ok {
		t.Fatal("expired challenge remains available")
	}
}
