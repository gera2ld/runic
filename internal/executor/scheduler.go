package executor

import (
	"context"
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"runic/internal/db"
)

type Scheduler struct {
	cron    *cron.Cron
	runner  *Runner
	db      *db.DB
	logDir  string
	entries map[string]cron.EntryID
	specs   map[string]string
	// missed records actions whose ticks fired while the runner was paused
	// for reload. They get one catch-up run each on Backfill.
	missed map[string]struct{}
	mu     sync.Mutex
}

func NewScheduler(runner *Runner, db *db.DB, logDir string) *Scheduler {
	return &Scheduler{
		cron:    cron.New(),
		runner:  runner,
		db:      db,
		logDir:  logDir,
		entries: make(map[string]cron.EntryID),
		specs:   make(map[string]string),
		missed:  make(map[string]struct{}),
	}
}

func (s *Scheduler) Start() {
	s.cron.Start()
	go s.syncLoop()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) syncLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Initial sync
	if err := s.Sync(); err != nil {
		log.Printf("[scheduler] initial sync failed: %v\n", err)
	}

	for range ticker.C {
		if err := s.Sync(); err != nil {
			log.Printf("[scheduler] sync failed: %v\n", err)
		}
	}
}

func (s *Scheduler) Sync() error {
	cfg := s.runner.Config()
	actions, err := ListActions(cfg.Actions, cfg.Timeout, s.db)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	newSpecs := make(map[string]string)
	for _, action := range actions {
		if action.Cron != "" {
			newSpecs[action.ID] = action.Cron
		}
	}

	// Remove old or changed actions
	for id, entryID := range s.entries {
		newSpec, exists := newSpecs[id]
		if !exists || newSpec != s.specs[id] {
			s.cron.Remove(entryID)
			delete(s.entries, id)
			delete(s.specs, id)
			log.Printf("[scheduler] removed/updating action: %s\n", id)
		}
	}

	// Add new or updated actions
	for id, spec := range newSpecs {
		if _, exists := s.entries[id]; !exists {
			actionID := id // capture for closure
			entryID, err := s.cron.AddFunc(spec, func() {
				log.Printf("[scheduler] triggering action: %s\n", actionID)
				_, err := s.runner.RunAction(context.Background(), s.db, s.logDir, actionID, "")
				if err != nil {
					if errors.Is(err, ErrPaused) {
						log.Printf("[scheduler] missed tick for %s during reload pause, will backfill on resume\n", actionID)
						s.mu.Lock()
						s.missed[actionID] = struct{}{}
						s.mu.Unlock()
						return
					}
					log.Printf("[scheduler] failed to trigger action %s: %v\n", actionID, err)
				}
			})
			if err != nil {
				log.Printf("[scheduler] failed to schedule action %s: %v\n", actionID, err)
				continue
			}
			s.entries[actionID] = entryID
			s.specs[actionID] = spec
			log.Printf("[scheduler] scheduled action: %s with spec: %s\n", actionID, spec)
		}
	}

	return nil
}

// Backfill queues one catch-up run for each action that missed ticks while
// the runner was paused. It must be called after Resume, runs against the
// current config (a removed action is skipped), and returns the ids it
// attempted, sorted.
func (s *Scheduler) Backfill() []string {
	s.mu.Lock()
	ids := make([]string, 0, len(s.missed))
	for id := range s.missed {
		ids = append(ids, id)
	}
	s.missed = make(map[string]struct{})
	s.mu.Unlock()

	sort.Strings(ids)
	for _, id := range ids {
		log.Printf("[scheduler] backfilling missed ticks for action: %s\n", id)
		if _, err := s.runner.RunAction(context.Background(), s.db, s.logDir, id, ""); err != nil {
			log.Printf("[scheduler] backfill failed for action %s: %v\n", id, err)
		}
	}
	return ids
}
