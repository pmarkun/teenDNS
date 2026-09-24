package digest

import (
	"context"
	"log"
	"time"

	"github.com/pmarkun/teendns/internal/config"
	"github.com/pmarkun/teendns/internal/mail"
	"github.com/pmarkun/teendns/internal/policy"
)

// ConfigSource gives the scheduler a read-only, point-in-time view of the
// live configuration, so newly registered houses and edited profiles are
// picked up without restarting the process.
type ConfigSource interface {
	Snapshot() config.Config
}

// Scheduler periodically sends the weekly digest to every house that has an
// email on file and whose cadence is due.
type Scheduler struct {
	Store    *Store
	Sender   mail.Sender
	Config   ConfigSource
	Interval time.Duration

	// Now defaults to time.Now; overridable in tests.
	Now func() time.Time
}

// Run blocks, checking every tick whether any house is due, until ctx is
// canceled.
func (scheduler *Scheduler) Run(ctx context.Context, tick time.Duration) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scheduler.checkAndSend()
		}
	}
}

func (scheduler *Scheduler) now() time.Time {
	if scheduler.Now != nil {
		return scheduler.Now()
	}
	return time.Now()
}

func (scheduler *Scheduler) checkAndSend() {
	now := scheduler.now()
	cfg := scheduler.Config.Snapshot()
	for _, house := range cfg.Houses {
		if house.Email == "" {
			continue
		}
		if !scheduler.Store.IsDue(house.ID, now, scheduler.Interval) {
			continue
		}
		profileIDs := profileIDsForHouse(cfg.Profiles, house.ID)
		report := scheduler.Store.Digest(profileIDs)
		html := RenderDigestEmail(house.Name, report)
		if err := scheduler.Sender.Send(house.Email, "Resumo semanal do teenDNS", html); err != nil {
			log.Printf("digest: falha ao enviar para a casa %s: %v", house.ID, err)
			continue
		}
		if err := scheduler.Store.CompleteDigest(house.ID, profileIDs, now); err != nil {
			log.Printf("digest: falha ao persistir estado após envio para a casa %s: %v", house.ID, err)
		}
	}
}

func profileIDsForHouse(profiles []policy.Profile, houseID string) []string {
	ids := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if profile.HouseID == houseID {
			ids = append(ids, profile.ID)
		}
	}
	return ids
}
