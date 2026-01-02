package codereview

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// PBStore implements ReviewStore using PocketBase
type PBStore struct {
	app *pocketbase.PocketBase
}

// NewPBStore creates a new PocketBase review store
func NewPBStore(app *pocketbase.PocketBase) *PBStore {
	return &PBStore{app: app}
}

// SaveReview saves a new review
func (s *PBStore) SaveReview(ctx context.Context, review Review) error {
	collection, err := s.app.FindCollectionByNameOrId("code_reviews")
	if err != nil {
		return fmt.Errorf("code_reviews collection not found: %w", err)
	}

	record := core.NewRecord(collection)

	// Set fields
	record.Set("review_type", string(review.ReviewType))
	record.Set("reviewer", review.Reviewer)
	record.Set("scope", string(review.Scope))
	record.Set("target", review.Target)
	record.Set("status", string(review.Status))
	record.Set("issues_found", review.IssuesFound)
	record.Set("files_reviewed", review.FilesReviewed)
	record.Set("summary", review.Summary)
	record.Set("started_at", review.StartedAt)

	if review.CompletedAt != nil {
		record.Set("completed_at", *review.CompletedAt)
	}

	record.Set("duration_seconds", review.DurationSeconds)

	// Set JSON fields
	if review.Issues != nil {
		issuesJSON, err := json.Marshal(review.Issues)
		if err != nil {
			return fmt.Errorf("failed to marshal issues: %w", err)
		}
		record.Set("issues", string(issuesJSON))
	}

	if review.Metadata != nil {
		metadataJSON, err := json.Marshal(review.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		record.Set("metadata", string(metadataJSON))
	}

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to save review record: %w", err)
	}

	// Update review ID
	review.ID = record.Id

	return nil
}

// UpdateReview updates an existing review
func (s *PBStore) UpdateReview(ctx context.Context, review Review) error {
	collection, err := s.app.FindCollectionByNameOrId("code_reviews")
	if err != nil {
		return fmt.Errorf("code_reviews collection not found: %w", err)
	}

	record, err := s.app.FindRecordById(collection.Id, review.ID)
	if err != nil {
		return fmt.Errorf("review not found: %w", err)
	}

	// Update fields
	record.Set("status", string(review.Status))
	record.Set("issues_found", review.IssuesFound)
	record.Set("files_reviewed", review.FilesReviewed)
	record.Set("summary", review.Summary)

	if review.CompletedAt != nil {
		record.Set("completed_at", *review.CompletedAt)
	}

	record.Set("duration_seconds", review.DurationSeconds)

	// Update JSON fields
	if review.Issues != nil {
		issuesJSON, err := json.Marshal(review.Issues)
		if err != nil {
			return fmt.Errorf("failed to marshal issues: %w", err)
		}
		record.Set("issues", string(issuesJSON))
	}

	if review.Metadata != nil {
		metadataJSON, err := json.Marshal(review.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		record.Set("metadata", string(metadataJSON))
	}

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to update review record: %w", err)
	}

	return nil
}

// GetReview retrieves a review by ID
func (s *PBStore) GetReview(ctx context.Context, id string) (*Review, error) {
	collection, err := s.app.FindCollectionByNameOrId("code_reviews")
	if err != nil {
		return nil, fmt.Errorf("code_reviews collection not found: %w", err)
	}

	record, err := s.app.FindRecordById(collection.Id, id)
	if err != nil {
		return nil, fmt.Errorf("review not found: %w", err)
	}

	return s.recordToReview(record)
}

// GetRecentReviews retrieves recent reviews
func (s *PBStore) GetRecentReviews(ctx context.Context, limit int) ([]Review, error) {
	collection, err := s.app.FindCollectionByNameOrId("code_reviews")
	if err != nil {
		return nil, fmt.Errorf("code_reviews collection not found: %w", err)
	}

	records, err := s.app.FindRecordsByFilter(
		collection.Id,
		"",
		"-started_at",
		limit,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query reviews: %w", err)
	}

	reviews := make([]Review, 0, len(records))
	for _, record := range records {
		review, err := s.recordToReview(record)
		if err != nil {
			continue // Skip invalid records
		}
		reviews = append(reviews, *review)
	}

	return reviews, nil
}

// GetReviewsByStatus retrieves reviews by status
func (s *PBStore) GetReviewsByStatus(ctx context.Context, status ReviewStatus) ([]Review, error) {
	collection, err := s.app.FindCollectionByNameOrId("code_reviews")
	if err != nil {
		return nil, fmt.Errorf("code_reviews collection not found: %w", err)
	}

	filter := fmt.Sprintf("status = '%s'", status)
	records, err := s.app.FindRecordsByFilter(
		collection.Id,
		filter,
		"-started_at",
		100,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query reviews: %w", err)
	}

	reviews := make([]Review, 0, len(records))
	for _, record := range records {
		review, err := s.recordToReview(record)
		if err != nil {
			continue
		}
		reviews = append(reviews, *review)
	}

	return reviews, nil
}

// recordToReview converts a PocketBase record to a Review
func (s *PBStore) recordToReview(record *core.Record) (*Review, error) {
	review := &Review{
		ID:              record.Id,
		ReviewType:      ReviewType(record.GetString("review_type")),
		Reviewer:        record.GetString("reviewer"),
		Scope:           ReviewScope(record.GetString("scope")),
		Target:          record.GetString("target"),
		Status:          ReviewStatus(record.GetString("status")),
		IssuesFound:     record.GetInt("issues_found"),
		FilesReviewed:   record.GetInt("files_reviewed"),
		Summary:         record.GetString("summary"),
		StartedAt:       record.GetDateTime("started_at").Time(),
		DurationSeconds: record.GetFloat("duration_seconds"),
	}

	// Parse completed_at
	if completedAtStr := record.GetString("completed_at"); completedAtStr != "" {
		completedAt := record.GetDateTime("completed_at").Time()
		review.CompletedAt = &completedAt
	}

	// Parse issues JSON
	if issuesStr := record.GetString("issues"); issuesStr != "" {
		var issues []Issue
		if err := json.Unmarshal([]byte(issuesStr), &issues); err == nil {
			review.Issues = issues
		}
	}

	// Parse metadata JSON
	if metadataStr := record.GetString("metadata"); metadataStr != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err == nil {
			review.Metadata = metadata
		}
	}

	return review, nil
}
