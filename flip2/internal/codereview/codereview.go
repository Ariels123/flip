package codereview

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// ReviewType defines the type of code review
type ReviewType string

const (
	ReviewTypePeriodic   ReviewType = "periodic"
	ReviewTypeOnDemand   ReviewType = "on_demand"
	ReviewTypePreCommit  ReviewType = "pre_commit"
)

// ReviewScope defines what code to review
type ReviewScope string

const (
	ScopeFull          ReviewScope = "full"
	ScopeRecentChanges ReviewScope = "recent_changes"
	ScopeFile          ReviewScope = "file"
	ScopeUncommitted   ReviewScope = "uncommitted"
)

// ReviewStatus defines the status of a review
type ReviewStatus string

const (
	StatusPending    ReviewStatus = "pending"
	StatusInProgress ReviewStatus = "in_progress"
	StatusCompleted  ReviewStatus = "completed"
	StatusFailed     ReviewStatus = "failed"
)

// Issue represents a single code review issue
type Issue struct {
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Severity    string `json:"severity"`    // "critical", "warning", "info"
	Category    string `json:"category"`    // "bug", "security", "performance", "style"
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// Review represents a code review session
type Review struct {
	ID             string       `json:"id"`
	ReviewType     ReviewType   `json:"review_type"`
	Reviewer       string       `json:"reviewer"`        // "gemini", "claude", etc.
	Scope          ReviewScope  `json:"scope"`
	Target         string       `json:"target,omitempty"` // File path or commit range
	Status         ReviewStatus `json:"status"`
	IssuesFound    int          `json:"issues_found"`
	FilesReviewed  int          `json:"files_reviewed"`
	Issues         []Issue      `json:"issues,omitempty"`
	Summary        string       `json:"summary,omitempty"`
	StartedAt      time.Time    `json:"started_at"`
	CompletedAt    *time.Time   `json:"completed_at,omitempty"`
	DurationSeconds float64     `json:"duration_seconds"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ReviewStore defines the interface for review persistence
type ReviewStore interface {
	SaveReview(ctx context.Context, review Review) error
	UpdateReview(ctx context.Context, review Review) error
	GetReview(ctx context.Context, id string) (*Review, error)
	GetRecentReviews(ctx context.Context, limit int) ([]Review, error)
	GetReviewsByStatus(ctx context.Context, status ReviewStatus) ([]Review, error)
}

// Reviewer defines the interface for code reviewers (LLM backends)
type Reviewer interface {
	Name() string
	ReviewCode(ctx context.Context, scope ReviewScope, target string) ([]Issue, string, error)
}

// Service manages code reviews
type Service struct {
	store     ReviewStore
	reviewers map[string]Reviewer
	logger    *slog.Logger
	repoPath  string
}

// NewService creates a new code review service
func NewService(store ReviewStore, repoPath string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		store:     store,
		reviewers: make(map[string]Reviewer),
		logger:    logger.WithGroup("codereview"),
		repoPath:  repoPath,
	}
}

// RegisterReviewer adds a reviewer to the service
func (s *Service) RegisterReviewer(reviewer Reviewer) {
	s.reviewers[reviewer.Name()] = reviewer
	s.logger.Info("Registered code reviewer", "reviewer", reviewer.Name())
}

// StartReview initiates a new code review
func (s *Service) StartReview(ctx context.Context, reviewType ReviewType, reviewerName string, scope ReviewScope, target string) (string, error) {
	reviewer, ok := s.reviewers[reviewerName]
	if !ok {
		return "", fmt.Errorf("reviewer not found: %s", reviewerName)
	}

	review := Review{
		ReviewType: reviewType,
		Reviewer:   reviewerName,
		Scope:      scope,
		Target:     target,
		Status:     StatusPending,
		StartedAt:  time.Now(),
	}

	if err := s.store.SaveReview(ctx, review); err != nil {
		return "", fmt.Errorf("failed to save review: %w", err)
	}

	s.logger.Info("Started code review",
		"review_id", review.ID,
		"type", reviewType,
		"reviewer", reviewerName,
		"scope", scope,
	)

	// Execute review asynchronously
	go s.executeReview(context.Background(), review.ID, reviewer, scope, target)

	return review.ID, nil
}

// executeReview runs the actual review process
func (s *Service) executeReview(ctx context.Context, reviewID string, reviewer Reviewer, scope ReviewScope, target string) {
	// Update status to in_progress
	review, err := s.store.GetReview(ctx, reviewID)
	if err != nil {
		s.logger.Error("Failed to get review", "review_id", reviewID, "error", err)
		return
	}

	review.Status = StatusInProgress
	if err := s.store.UpdateReview(ctx, *review); err != nil {
		s.logger.Error("Failed to update review status", "review_id", reviewID, "error", err)
		return
	}

	startTime := time.Now()

	// Execute the review
	issues, summary, err := reviewer.ReviewCode(ctx, scope, target)
	if err != nil {
		s.logger.Error("Review failed",
			"review_id", reviewID,
			"reviewer", reviewer.Name(),
			"error", err,
		)

		review.Status = StatusFailed
		review.Summary = fmt.Sprintf("Review failed: %v", err)
		completedAt := time.Now()
		review.CompletedAt = &completedAt
		review.DurationSeconds = time.Since(startTime).Seconds()
		_ = s.store.UpdateReview(ctx, *review)
		return
	}

	// Update review with results
	review.Status = StatusCompleted
	review.Issues = issues
	review.IssuesFound = len(issues)
	review.Summary = summary
	review.FilesReviewed = s.countFilesReviewed(issues)
	completedAt := time.Now()
	review.CompletedAt = &completedAt
	review.DurationSeconds = time.Since(startTime).Seconds()

	if err := s.store.UpdateReview(ctx, *review); err != nil {
		s.logger.Error("Failed to update review results", "review_id", reviewID, "error", err)
		return
	}

	s.logger.Info("Review completed",
		"review_id", reviewID,
		"issues_found", review.IssuesFound,
		"files_reviewed", review.FilesReviewed,
		"duration", review.DurationSeconds,
	)
}

// countFilesReviewed counts unique files from issues
func (s *Service) countFilesReviewed(issues []Issue) int {
	files := make(map[string]bool)
	for _, issue := range issues {
		if issue.File != "" {
			files[issue.File] = true
		}
	}
	return len(files)
}

// GetRecentChanges returns git diff of recent changes
func (s *Service) GetRecentChanges(ctx context.Context, since time.Duration) (string, error) {
	// Get commits since time
	cmd := exec.CommandContext(ctx, "git", "log",
		fmt.Sprintf("--since=%s", since.String()),
		"--name-status",
		"--oneline",
	)
	cmd.Dir = s.repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git log: %w", err)
	}

	return string(output), nil
}

// GetUncommittedChanges returns git diff of uncommitted changes
func (s *Service) GetUncommittedChanges(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD")
	cmd.Dir = s.repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git diff: %w", err)
	}

	return string(output), nil
}

// GetChangedFiles returns list of changed files
func (s *Service) GetChangedFiles(ctx context.Context, since time.Duration) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff",
		"--name-only",
		fmt.Sprintf("HEAD@{%s}", since.String()),
		"HEAD",
	)
	cmd.Dir = s.repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	return files, nil
}
