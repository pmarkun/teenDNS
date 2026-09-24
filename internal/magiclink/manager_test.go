package magiclink

import (
	"testing"
	"time"
)

func TestIssueAndRedeemGrantsSessionForTheSameHouse(t *testing.T) {
	manager := NewManager(time.Minute, time.Hour)
	token, err := manager.IssueLink("house-1")
	if err != nil {
		t.Fatal(err)
	}
	houseID, sessionToken, ok := manager.Redeem(token)
	if !ok || houseID != "house-1" || sessionToken == "" {
		t.Fatalf("unexpected redeem result: houseID=%q sessionToken=%q ok=%v", houseID, sessionToken, ok)
	}
	resolved, ok := manager.HouseID(sessionToken)
	if !ok || resolved != "house-1" {
		t.Fatalf("unexpected session lookup: %q, %v", resolved, ok)
	}
}

func TestRedeemConsumesTheLinkOnce(t *testing.T) {
	manager := NewManager(time.Minute, time.Hour)
	token, _ := manager.IssueLink("house-1")
	if _, _, ok := manager.Redeem(token); !ok {
		t.Fatal("expected first redeem to succeed")
	}
	if _, _, ok := manager.Redeem(token); ok {
		t.Fatal("expected second redeem of the same link to fail")
	}
}

func TestRedeemRejectsUnknownToken(t *testing.T) {
	manager := NewManager(time.Minute, time.Hour)
	if _, _, ok := manager.Redeem("does-not-exist"); ok {
		t.Fatal("expected unknown token to be rejected")
	}
}

func TestLinkExpires(t *testing.T) {
	manager := NewManager(time.Minute, time.Hour)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	token, _ := manager.IssueLink("house-1")
	now = now.Add(2 * time.Minute)
	if _, _, ok := manager.Redeem(token); ok {
		t.Fatal("expected expired link to be rejected")
	}
}

func TestSessionExpires(t *testing.T) {
	manager := NewManager(time.Minute, time.Hour)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	token, _ := manager.IssueLink("house-1")
	_, sessionToken, ok := manager.Redeem(token)
	if !ok {
		t.Fatal("expected redeem to succeed")
	}
	now = now.Add(2 * time.Hour)
	if _, ok := manager.HouseID(sessionToken); ok {
		t.Fatal("expected expired session to be rejected")
	}
}

func TestHouseIDRejectsUnknownSession(t *testing.T) {
	manager := NewManager(time.Minute, time.Hour)
	if _, ok := manager.HouseID("does-not-exist"); ok {
		t.Fatal("expected unknown session token to be rejected")
	}
}
