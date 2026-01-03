package budget

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Scheduler runs periodic budget reset checks
type Scheduler struct {
	tracker  *Tracker
	logger   *slog.Logger
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  bool
	mu       sync.Mutex
}

// NewScheduler creates a new budget reset scheduler
func NewScheduler(tracker *Tracker, logger *slog.Logger, interval time.Duration) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = 5 * time.Minute // Default to 5 minute checks
	}
	return &Scheduler{
		tracker:  tracker,
		logger:   logger.WithGroup("budget.scheduler"),
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the background scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run(ctx)

	s.logger.Info("Budget scheduler started", "interval", s.interval)
	return nil
}

// Stop halts the background scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	s.wg.Wait()
	s.logger.Info("Budget scheduler stopped")
}

// IsRunning returns whether the scheduler is currently running
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// run is the main scheduler loop
func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run an immediate check on start
	s.checkBudgets(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Budget scheduler context cancelled")
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkBudgets(ctx)
		}
	}
}

// checkBudgets performs the periodic budget reset check
func (s *Scheduler) checkBudgets(ctx context.Context) {
	start := time.Now()

	if err := s.tracker.CheckAndResetBudgets(ctx); err != nil {
		s.logger.Error("Failed to check and reset budgets", "error", err)
		return
	}

	elapsed := time.Since(start)
	s.logger.Debug("Budget check completed", "duration", elapsed)
}

// RunOnce performs a single budget check without starting the scheduler
func (s *Scheduler) RunOnce(ctx context.Context) error {
	return s.tracker.CheckAndResetBudgets(ctx)
}
