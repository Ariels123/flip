package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/robfig/cron/v3"
)

// Scheduler runs periodic jobs
type Scheduler struct {
	pb      *pocketbase.PocketBase
	cron    *cron.Cron
	jobs    map[string]*Job
	logger  *slog.Logger
	mu      sync.Mutex
	running bool
}

// Job represents a scheduled task
type Job struct {
	ID         string
	Name       string
	Cron       string
	Handler    func(context.Context) error
	Enabled    bool
	LastRun    time.Time
	NextRun    time.Time
	LastResult string
}

// New creates a new Scheduler
func New(pb *pocketbase.PocketBase, logger *slog.Logger) *Scheduler {
	// Use the provided logger, and add a "scheduler" group
	schedulerLogger := logger.WithGroup("scheduler")

	return &Scheduler{
		pb:     pb,
		cron:   cron.New(cron.WithSeconds()), // Enable seconds field
		jobs:   make(map[string]*Job),
		logger: schedulerLogger,
	}
}

// RegisterJob adds a new job to the scheduler
func (s *Scheduler) RegisterJob(name string, cronExpr string, handler func(context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.jobs[name] = &Job{
		Name:    name,
		Cron:    cronExpr,
		Handler: handler,
		Enabled: true,
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("Starting scheduler")

	// Schedule all registered jobs
	for name, job := range s.jobs {
		if !job.Enabled {
			continue
		}

		jobName := name // Capture loop variable
		jobRef := job

		entryID, err := s.cron.AddFunc(job.Cron, func() {
			s.runJob(jobName, jobRef)
		})

		if err != nil {
			s.logger.Error("Failed to schedule job", "name", name, "cron", job.Cron, "error", err)
		} else {
			s.logger.Info("Scheduled job", "name", name, "cron", job.Cron, "entryID", entryID)
		}
	}

	s.cron.Start()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.running {
		return
	}
	
	s.logger.Info("Stopping scheduler")
	s.cron.Stop()
	s.running = false
}

func (s *Scheduler) runJob(name string, job *Job) {
	s.logger.Info("Running job", "name", name)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	job.LastRun = time.Now()
	
	err := job.Handler(ctx)
	
	if err != nil {
		s.logger.Error("Job failed", "name", name, "error", err)
		job.LastResult = fmt.Sprintf("failed: %v", err)
	} else {
		s.logger.Info("Job completed", "name", name)
		job.LastResult = "success"
	}
	
	// TODO: Update job status in PocketBase 'jobs' collection
}
