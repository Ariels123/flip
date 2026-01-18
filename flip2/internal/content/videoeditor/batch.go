// Package videoeditor - Batch processing system (Phase 4)
// Enables high-volume video generation from data sources
package videoeditor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"flip2/internal/logger"
)

// BatchProcessor handles batch video generation
type BatchProcessor struct {
	editor          *Editor
	templateManager *TemplateManager
	logger          *logger.Logger

	// Concurrency control
	maxConcurrent int
	semaphore     chan struct{}

	// Progress tracking
	progress *BatchProgress
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(editor *Editor, tm *TemplateManager, maxConcurrent int, log *logger.Logger) *BatchProcessor {
	if maxConcurrent <= 0 {
		maxConcurrent = 4 // Default to 4 concurrent jobs
	}

	return &BatchProcessor{
		editor:          editor,
		templateManager: tm,
		logger:          log,
		maxConcurrent:   maxConcurrent,
		semaphore:       make(chan struct{}, maxConcurrent),
		progress:        NewBatchProgress(),
	}
}

// BatchJob represents a single video generation job
type BatchJob struct {
	// ID uniquely identifies this job
	ID string

	// TemplateName is the template to use
	TemplateName string

	// Variables for template substitution
	Variables map[string]interface{}

	// OutputPath for the generated video
	OutputPath string

	// Priority (higher = more important)
	Priority int

	// Metadata for tracking
	Metadata map[string]interface{}
}

// BatchResult contains the result of a batch job
type BatchResult struct {
	// Job is the original job
	Job *BatchJob

	// Success indicates if the job completed successfully
	Success bool

	// OutputPath is the path to the generated video (if successful)
	OutputPath string

	// Error contains the error message (if failed)
	Error string

	// Duration is how long the job took
	Duration time.Duration

	// StartTime when the job started
	StartTime time.Time

	// EndTime when the job finished
	EndTime time.Time

	// Metrics contains additional metrics
	Metrics map[string]interface{}
}

// BatchProgress tracks overall batch progress
type BatchProgress struct {
	mu sync.RWMutex

	// TotalJobs is the total number of jobs
	TotalJobs int64

	// CompletedJobs is the number of completed jobs
	CompletedJobs int64

	// FailedJobs is the number of failed jobs
	FailedJobs int64

	// ActiveJobs is the number of currently running jobs
	ActiveJobs int64

	// StartTime when the batch started
	StartTime time.Time

	// JobResults stores results for completed jobs
	JobResults map[string]*BatchResult
}

// NewBatchProgress creates a new progress tracker
func NewBatchProgress() *BatchProgress {
	return &BatchProgress{
		JobResults: make(map[string]*BatchResult),
		StartTime:  time.Now(),
	}
}

// GetStats returns current progress statistics
func (bp *BatchProgress) GetStats() BatchStats {
	// Use atomic loads for counters to match atomic stores
	totalJobs := atomic.LoadInt64(&bp.TotalJobs)
	completedJobs := atomic.LoadInt64(&bp.CompletedJobs)
	failedJobs := atomic.LoadInt64(&bp.FailedJobs)
	activeJobs := atomic.LoadInt64(&bp.ActiveJobs)

	elapsed := time.Since(bp.StartTime)
	completionRate := float64(0)
	if totalJobs > 0 {
		completionRate = float64(completedJobs) / float64(totalJobs) * 100
	}

	estimatedTimeRemaining := time.Duration(0)
	if completedJobs > 0 {
		avgTimePerJob := elapsed / time.Duration(completedJobs)
		remainingJobs := totalJobs - completedJobs
		estimatedTimeRemaining = avgTimePerJob * time.Duration(remainingJobs)
	}

	return BatchStats{
		TotalJobs:              totalJobs,
		CompletedJobs:          completedJobs,
		FailedJobs:             failedJobs,
		ActiveJobs:             activeJobs,
		CompletionRate:         completionRate,
		ElapsedTime:            elapsed,
		EstimatedTimeRemaining: estimatedTimeRemaining,
	}
}

// BatchStats contains batch statistics
type BatchStats struct {
	TotalJobs              int64
	CompletedJobs          int64
	FailedJobs             int64
	ActiveJobs             int64
	CompletionRate         float64
	ElapsedTime            time.Duration
	EstimatedTimeRemaining time.Duration
}

// ProcessBatch processes a batch of jobs
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, jobs []*BatchJob) ([]*BatchResult, error) {
	if bp.logger != nil {
		bp.logger.InfoCtx(ctx, "Starting batch processing",
			"total_jobs", len(jobs),
			"max_concurrent", bp.maxConcurrent)
	}

	// Initialize progress
	bp.progress = NewBatchProgress()
	atomic.StoreInt64(&bp.progress.TotalJobs, int64(len(jobs)))

	// Create results channel
	results := make([]*BatchResult, 0, len(jobs))
	resultChan := make(chan *BatchResult, len(jobs))
	errorChan := make(chan error, 1)

	// Process jobs concurrently
	var wg sync.WaitGroup

	for _, job := range jobs {
		wg.Add(1)

		go func(j *BatchJob) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case bp.semaphore <- struct{}{}:
				defer func() { <-bp.semaphore }()
			case <-ctx.Done():
				resultChan <- &BatchResult{
					Job:     j,
					Success: false,
					Error:   "context cancelled",
				}
				return
			}

			// Update active count
			atomic.AddInt64(&bp.progress.ActiveJobs, 1)
			defer atomic.AddInt64(&bp.progress.ActiveJobs, -1)

			// Process job
			result := bp.processJob(ctx, j)
			resultChan <- result

			// Update progress
			if result.Success {
				atomic.AddInt64(&bp.progress.CompletedJobs, 1)
			} else {
				atomic.AddInt64(&bp.progress.FailedJobs, 1)
			}

			// Store result
			bp.progress.mu.Lock()
			bp.progress.JobResults[j.ID] = result
			bp.progress.mu.Unlock()
		}(job)
	}

	// Wait for all jobs to complete
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	if bp.logger != nil {
		stats := bp.progress.GetStats()
		bp.logger.InfoCtx(ctx, "Batch processing completed",
			"total_jobs", stats.TotalJobs,
			"completed", stats.CompletedJobs,
			"failed", stats.FailedJobs,
			"elapsed", stats.ElapsedTime)
	}

	return results, nil
}

// processJob processes a single job
func (bp *BatchProcessor) processJob(ctx context.Context, job *BatchJob) *BatchResult {
	startTime := time.Now()

	result := &BatchResult{
		Job:       job,
		StartTime: startTime,
		Metrics:   make(map[string]interface{}),
	}

	if bp.logger != nil {
		bp.logger.InfoCtx(ctx, "Processing job",
			"job_id", job.ID,
			"template", job.TemplateName)
	}

	// Load template
	template, err := bp.templateManager.Load(ctx, job.TemplateName)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to load template: %v", err)
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result
	}

	// Create timeline executor
	ops := NewVideoOperations(bp.editor.ffmpegPath, bp.editor.ffprobePath, bp.editor.tempDir)
	te := NewTimelineExecutor(ops, bp.editor.tempDir, bp.logger)

	// Execute template
	outputPath, err := te.Execute(ctx, template, job.Variables, job.OutputPath)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to execute template: %v", err)
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result
	}

	result.Success = true
	result.OutputPath = outputPath
	result.EndTime = time.Now()
	result.Duration = time.Since(startTime)

	if bp.logger != nil {
		bp.logger.InfoCtx(ctx, "Job completed",
			"job_id", job.ID,
			"output", outputPath,
			"duration", result.Duration)
	}

	return result
}

// GetProgress returns the current batch progress
func (bp *BatchProcessor) GetProgress() BatchStats {
	return bp.progress.GetStats()
}

// RetryFailed retries all failed jobs from a previous batch
func (bp *BatchProcessor) RetryFailed(ctx context.Context, previousResults []*BatchResult) ([]*BatchResult, error) {
	failedJobs := make([]*BatchJob, 0)

	for _, result := range previousResults {
		if !result.Success {
			failedJobs = append(failedJobs, result.Job)
		}
	}

	if len(failedJobs) == 0 {
		if bp.logger != nil {
			bp.logger.InfoCtx(ctx, "No failed jobs to retry")
		}
		return []*BatchResult{}, nil
	}

	if bp.logger != nil {
		bp.logger.InfoCtx(ctx, "Retrying failed jobs", "count", len(failedJobs))
	}

	return bp.ProcessBatch(ctx, failedJobs)
}

// ProcessWithRetry processes a batch with automatic retry on failure
func (bp *BatchProcessor) ProcessWithRetry(ctx context.Context, jobs []*BatchJob, maxRetries int) ([]*BatchResult, error) {
	if maxRetries < 0 {
		maxRetries = 0
	}

	// Initial batch
	results, err := bp.ProcessBatch(ctx, jobs)
	if err != nil {
		return nil, err
	}

	// Retry failed jobs
	for retry := 0; retry < maxRetries; retry++ {
		failedCount := int64(0)
		for _, result := range results {
			if !result.Success {
				failedCount++
			}
		}

		if failedCount == 0 {
			break // All jobs succeeded
		}

		if bp.logger != nil {
			bp.logger.InfoCtx(ctx, "Retrying failed jobs",
				"retry", retry+1,
				"max_retries", maxRetries,
				"failed_count", failedCount)
		}

		// Retry failed jobs
		retryResults, err := bp.RetryFailed(ctx, results)
		if err != nil {
			return results, err
		}

		// Update results with retry outcomes
		retryMap := make(map[string]*BatchResult)
		for _, r := range retryResults {
			retryMap[r.Job.ID] = r
		}

		for i, result := range results {
			if !result.Success {
				if retryResult, ok := retryMap[result.Job.ID]; ok {
					results[i] = retryResult
				}
			}
		}
	}

	return results, nil
}

// GenerateBatchReport generates a summary report for batch results
func GenerateBatchReport(results []*BatchResult) *BatchReport {
	report := &BatchReport{
		TotalJobs:      int64(len(results)),
		SuccessfulJobs: 0,
		FailedJobs:     0,
		TotalDuration:  0,
		AverageDuration: 0,
		FailureReasons: make(map[string]int),
	}

	for _, result := range results {
		if result.Success {
			report.SuccessfulJobs++
		} else {
			report.FailedJobs++
			report.FailureReasons[result.Error]++
		}

		report.TotalDuration += result.Duration
	}

	if report.TotalJobs > 0 {
		report.AverageDuration = report.TotalDuration / time.Duration(report.TotalJobs)
		report.SuccessRate = float64(report.SuccessfulJobs) / float64(report.TotalJobs) * 100
	}

	return report
}

// BatchReport contains batch processing statistics
type BatchReport struct {
	TotalJobs       int64
	SuccessfulJobs  int64
	FailedJobs      int64
	SuccessRate     float64
	TotalDuration   time.Duration
	AverageDuration time.Duration
	FailureReasons  map[string]int
}
