package executor

import (
	"context"
	"log"
	"sync"
	"time"

	"runic/internal/db"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron      *cron.Cron
	runner    *Runner
	db        *db.DB
	actionDir string
	logDir    string
	entries   map[string]cron.EntryID
	specs     map[string]string
	mu        sync.Mutex
}

func NewScheduler(runner *Runner, db *db.DB, actionDir, logDir string) *Scheduler {
	return &Scheduler{
		cron:      cron.New(),
		runner:    runner,
		db:        db,
		actionDir: actionDir,
		logDir:    logDir,
		entries:   make(map[string]cron.EntryID),
		specs:     make(map[string]string),
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
	actions, err := ListActions(s.actionDir, s.runner.cfg.Timeout, s.db)
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
				_, err := s.runner.RunAction(context.Background(), s.db, s.logDir, s.actionDir, actionID, "")
				if err != nil {
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
