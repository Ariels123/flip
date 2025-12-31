package supervisor

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// RestartStrategy defines how to handle worker failures
type RestartStrategy int

const (
	// Permanent workers are always restarted
	Permanent RestartStrategy = iota
	// Transient workers are restarted only on abnormal exit
	Transient
	// Temporary workers are never restarted
	Temporary
)

// Worker represents a supervised goroutine
type Worker interface {
	// Start begins the worker (blocks until done or context cancelled)
	Start(ctx context.Context) error
	// Stop gracefully stops the worker
	Stop() error
	// Name returns the worker name for logging
	Name() string
}

// WorkerSpec defines how to supervise a worker
type WorkerSpec struct {
	Worker        Worker
	Strategy      RestartStrategy
	MaxRestarts   int           // Max restarts within RestartWindow
	RestartWindow time.Duration // Time window for counting restarts
}

// Supervisor manages a tree of workers with restart policies
type Supervisor struct {
	name      string
	workers   []*workerState
	logger    *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	intensity int           // Max total restarts across all workers
	period    time.Duration // Within this period
	restarts  []time.Time   // Global restart history
	stopped   bool
}

type workerState struct {
	spec     WorkerSpec
	restarts []time.Time
	running  bool
	mu       sync.Mutex
}

// New creates a new supervisor
func New(name string, logger *slog.Logger, intensity int, period time.Duration) *Supervisor {
	return &Supervisor{
		name:      name,
		logger:    logger.With("supervisor", name),
		workers:   make([]*workerState, 0),
		intensity: intensity,
		period:    period,
		restarts:  make([]time.Time, 0),
	}
}

// AddWorker adds a worker to supervision
func (s *Supervisor) AddWorker(spec WorkerSpec) {
	if spec.MaxRestarts == 0 {
		spec.MaxRestarts = 5 // Default: 5 restarts
	}
	if spec.RestartWindow == 0 {
		spec.RestartWindow = 1 * time.Minute // Default: within 1 minute
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.workers = append(s.workers, &workerState{
		spec:     spec,
		restarts: make([]time.Time, 0),
		running:  false,
	})

	s.logger.Info("Added worker", "worker", spec.Worker.Name(), "strategy", spec.Strategy)
}

// Start begins supervising all workers
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.stopped = false
	s.mu.Unlock()

	s.logger.Info("Starting supervisor", "workers", len(s.workers))

	for _, ws := range s.workers {
		s.wg.Add(1)
		go s.superviseWorker(ws)
	}

	return nil
}

// Stop gracefully stops all workers
func (s *Supervisor) Stop() error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()

	s.logger.Info("Stopping supervisor")

	// Stop all workers
	for _, ws := range s.workers {
		ws.mu.Lock()
		if ws.running {
			if err := ws.spec.Worker.Stop(); err != nil {
				s.logger.Error("Failed to stop worker", "worker", ws.spec.Worker.Name(), "error", err)
			}
		}
		ws.mu.Unlock()
	}

	// Wait for supervision goroutines to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Supervisor stopped gracefully")
	case <-time.After(10 * time.Second):
		s.logger.Warn("Supervisor stop timed out")
	}

	return nil
}

// superviseWorker monitors a single worker and restarts it according to policy
func (s *Supervisor) superviseWorker(ws *workerState) {
	defer s.wg.Done()

	workerName := ws.spec.Worker.Name()
	s.logger.Info("Supervising worker", "worker", workerName, "strategy", strategyName(ws.spec.Strategy))

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debug("Supervisor context done, stopping worker supervision", "worker", workerName)
			return
		default:
		}

		// Mark as running
		ws.mu.Lock()
		ws.running = true
		ws.mu.Unlock()

		// Start the worker (blocks until it exits)
		s.logger.Debug("Starting worker", "worker", workerName)
		err := ws.spec.Worker.Start(s.ctx)

		// Mark as not running
		ws.mu.Lock()
		ws.running = false
		ws.mu.Unlock()

		// Check if we're shutting down
		select {
		case <-s.ctx.Done():
			s.logger.Debug("Worker stopped due to shutdown", "worker", workerName)
			return
		default:
		}

		// Worker exited unexpectedly - decide whether to restart
		if err != nil {
			s.logger.Warn("Worker exited with error", "worker", workerName, "error", err)
		} else {
			s.logger.Info("Worker exited normally", "worker", workerName)
		}

		shouldRestart := s.shouldRestart(ws, err)
		if !shouldRestart {
			s.logger.Info("Not restarting worker per strategy", "worker", workerName, "strategy", strategyName(ws.spec.Strategy))
			return
		}

		// Check restart limits
		if !s.checkRestartLimits(ws) {
			s.logger.Error("Worker exceeded restart limits", "worker", workerName,
				"max_restarts", ws.spec.MaxRestarts, "window", ws.spec.RestartWindow)
			s.escalate(ws)
			return
		}

		// Record restart
		now := time.Now()
		ws.mu.Lock()
		ws.restarts = append(ws.restarts, now)
		restartCount := len(ws.restarts)
		ws.mu.Unlock()

		s.mu.Lock()
		s.restarts = append(s.restarts, now)
		s.mu.Unlock()

		// Calculate backoff
		backoff := s.calculateBackoff(restartCount)
		s.logger.Warn("Restarting worker", "worker", workerName, "backoff", backoff, "restart_count", restartCount)

		// Wait with backoff before restarting
		select {
		case <-time.After(backoff):
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Supervisor) shouldRestart(ws *workerState, err error) bool {
	switch ws.spec.Strategy {
	case Permanent:
		return true // Always restart
	case Transient:
		return err != nil // Restart only on error
	case Temporary:
		return false // Never restart
	default:
		return false
	}
}

func (s *Supervisor) checkRestartLimits(ws *workerState) bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-ws.spec.RestartWindow)

	// Count recent restarts
	count := 0
	for _, t := range ws.restarts {
		if t.After(cutoff) {
			count++
		}
	}

	return count < ws.spec.MaxRestarts
}

func (s *Supervisor) calculateBackoff(restartCount int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, max 30s
	backoff := time.Duration(1<<uint(restartCount-1)) * time.Second
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}
	if backoff < time.Second {
		backoff = time.Second
	}
	return backoff
}

func (s *Supervisor) escalate(ws *workerState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-s.period)

	// Count global restarts
	count := 0
	for _, t := range s.restarts {
		if t.After(cutoff) {
			count++
		}
	}

	if count >= s.intensity {
		s.logger.Error("Supervisor intensity exceeded, shutting down all workers",
			"restarts", count, "period", s.period, "intensity", s.intensity)

		if s.cancel != nil {
			s.cancel()
		}
	}
}

func strategyName(s RestartStrategy) string {
	switch s {
	case Permanent:
		return "permanent"
	case Transient:
		return "transient"
	case Temporary:
		return "temporary"
	default:
		return "unknown"
	}
}

// WorkerCount returns the number of supervised workers
func (s *Supervisor) WorkerCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.workers)
}

// RunningWorkers returns the count of currently running workers
func (s *Supervisor) RunningWorkers() int {
	count := 0
	for _, ws := range s.workers {
		ws.mu.Lock()
		if ws.running {
			count++
		}
		ws.mu.Unlock()
	}
	return count
}
