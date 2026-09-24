package digest

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/gateway"
	"github.com/pmarkun/teendns/internal/policy"
)

type fakeConfigSource struct {
	cfg config.Config
}

func (f fakeConfigSource) Snapshot() config.Config { return f.cfg }

type fakeSender struct {
	sent []struct{ to, subject, html string }
	fail bool
}

func (f *fakeSender) Send(to, subject, html string) error {
	if f.fail {
		return errSendFailed
	}
	f.sent = append(f.sent, struct{ to, subject, html string }{to, subject, html})
	return nil
}

type sentinelError string

func (e sentinelError) Error() string { return string(e) }

const errSendFailed = sentinelError("send failed")

func TestSchedulerSendsDigestOnlyForHousesWithEmailAndDue(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.Write(gateway.Event{ProfileID: "profile-1", Action: policy.ActionObserve, MatchedDomain: "discord.com", GroupName: "Discord", Category: "messaging_and_communities", Timestamp: time.Now()})

	cfg := config.Config{
		Houses: []config.House{
			{ID: "house-1", Name: "Casa Silva", Email: "responsavel@example.com"},
			{ID: "house-2", Name: "Casa sem e-mail"},
		},
		Profiles: []policy.Profile{
			{ID: "profile-1", HouseID: "house-1"},
		},
	}
	sender := &fakeSender{}
	scheduler := &Scheduler{
		Store:    store,
		Sender:   sender,
		Config:   fakeConfigSource{cfg: cfg},
		Interval: 7 * 24 * time.Hour,
	}

	scheduler.checkAndSend()

	if len(sender.sent) != 1 {
		t.Fatalf("expected exactly one email sent, got %d", len(sender.sent))
	}
	if sender.sent[0].to != "responsavel@example.com" {
		t.Fatalf("unexpected recipient %q", sender.sent[0].to)
	}
	if !store.IsDue("house-2", time.Now(), time.Hour) {
		t.Fatal("house without email should not be marked as sent")
	}
	if store.IsDue("house-1", time.Now(), scheduler.Interval) {
		t.Fatal("house-1 should be marked as sent and not due again immediately")
	}
	if report := store.Digest([]string{"profile-1"}); len(report) != 0 {
		t.Fatalf("expected entries cleared after successful send, got %+v", report)
	}
}

func TestSchedulerSkipsHouseNotYetDue(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := store.CompleteDigest("house-1", nil, now); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Houses: []config.House{{ID: "house-1", Email: "a@example.com"}}}
	sender := &fakeSender{}
	scheduler := &Scheduler{
		Store:    store,
		Sender:   sender,
		Config:   fakeConfigSource{cfg: cfg},
		Interval: 7 * 24 * time.Hour,
		Now:      func() time.Time { return now.Add(time.Hour) },
	}

	scheduler.checkAndSend()

	if len(sender.sent) != 0 {
		t.Fatalf("expected no email sent before cadence elapsed, got %d", len(sender.sent))
	}
}

func TestSchedulerDoesNotResetOnSendFailure(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.Write(gateway.Event{ProfileID: "profile-1", Action: policy.ActionObserve, MatchedDomain: "discord.com", Timestamp: time.Now()})

	cfg := config.Config{
		Houses:   []config.House{{ID: "house-1", Email: "a@example.com"}},
		Profiles: []policy.Profile{{ID: "profile-1", HouseID: "house-1"}},
	}
	sender := &fakeSender{fail: true}
	scheduler := &Scheduler{Store: store, Sender: sender, Config: fakeConfigSource{cfg: cfg}, Interval: time.Hour}

	scheduler.checkAndSend()

	if report := store.Digest([]string{"profile-1"}); len(report) != 1 {
		t.Fatalf("expected entries to survive a failed send, got %+v", report)
	}
	if !store.IsDue("house-1", time.Now(), scheduler.Interval) {
		t.Fatal("house should remain due after a failed send")
	}
}
