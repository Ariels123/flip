package supervisor

import (
	"context"
	"log/slog"

	"flip2/internal/executor"
	"flip2/internal/scheduler"
	"flip2/internal/sync"
)

// ExecutorWorker wraps the Executor for supervision
type ExecutorWorker struct {
	executor *executor.Executor
	logger   *slog.Logger
	cancel   context.CancelFunc
}

// NewExecutorWorker creates a supervised executor worker
func NewExecutorWorker(exec *executor.Executor, logger *slog.Logger) *ExecutorWorker {
	return &ExecutorWorker{
		executor: exec,
		logger:   logger.With("worker", "executor"),
	}
}

func (w *ExecutorWorker) Start(ctx context.Context) error {
	w.logger.Info("ExecutorWorker starting")

	// Create cancellable context for this run
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	// Start the executor (non-blocking, spawns internal goroutines)
	w.executor.Start()

	// Block until context is cancelled
	<-runCtx.Done()

	w.logger.Info("ExecutorWorker stopping")
	return runCtx.Err()
}

func (w *ExecutorWorker) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}

func (w *ExecutorWorker) Name() string {
	return "executor"
}

// SchedulerWorker wraps the Scheduler for supervision
type SchedulerWorker struct {
	scheduler *scheduler.Scheduler
	logger    *slog.Logger
	cancel    context.CancelFunc
}

// NewSchedulerWorker creates a supervised scheduler worker
func NewSchedulerWorker(sched *scheduler.Scheduler, logger *slog.Logger) *SchedulerWorker {
	return &SchedulerWorker{
		scheduler: sched,
		logger:    logger.With("worker", "scheduler"),
	}
}

func (w *SchedulerWorker) Start(ctx context.Context) error {
	w.logger.Info("SchedulerWorker starting")

	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	// Start the scheduler (non-blocking, uses internal cron)
	w.scheduler.Start()

	// Block until context is cancelled
	<-runCtx.Done()

	w.logger.Info("SchedulerWorker stopping")
	w.scheduler.Stop()
	return runCtx.Err()
}

func (w *SchedulerWorker) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}

func (w *SchedulerWorker) Name() string {
	return "scheduler"
}

// ReplicatorWorker wraps the Replicator for supervision
type ReplicatorWorker struct {
	replicator *sync.Replicator
	logger     *slog.Logger
	cancel     context.CancelFunc
}

// NewReplicatorWorker creates a supervised replicator worker
func NewReplicatorWorker(repl *sync.Replicator, logger *slog.Logger) *ReplicatorWorker {
	return &ReplicatorWorker{
		replicator: repl,
		logger:     logger.With("worker", "replicator"),
	}
}

func (w *ReplicatorWorker) Start(ctx context.Context) error {
	w.logger.Info("ReplicatorWorker starting")

	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	// Replicator is driven by scheduler jobs, not its own loop
	// We just monitor the context here
	<-runCtx.Done()

	w.logger.Info("ReplicatorWorker stopping")
	return runCtx.Err()
}

func (w *ReplicatorWorker) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}

func (w *ReplicatorWorker) Name() string {
	return "replicator"
}

// GenericWorker wraps any function as a supervised worker
type GenericWorker struct {
	name   string
	runFn  func(ctx context.Context) error
	stopFn func() error
	logger *slog.Logger
}

// NewGenericWorker creates a worker from functions
func NewGenericWorker(name string, runFn func(ctx context.Context) error, stopFn func() error, logger *slog.Logger) *GenericWorker {
	return &GenericWorker{
		name:   name,
		runFn:  runFn,
		stopFn: stopFn,
		logger: logger.With("worker", name),
	}
}

func (w *GenericWorker) Start(ctx context.Context) error {
	w.logger.Info("GenericWorker starting")
	err := w.runFn(ctx)
	w.logger.Info("GenericWorker stopped", "error", err)
	return err
}

func (w *GenericWorker) Stop() error {
	if w.stopFn != nil {
		return w.stopFn()
	}
	return nil
}

func (w *GenericWorker) Name() string {
	return w.name
}
