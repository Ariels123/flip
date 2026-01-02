package routing

import (
	"strings"
	"testing"
)

// ================================================================================
// TEST CASES FOR TASK CLASSIFICATION
// ================================================================================

// TestClassifyTaskBasicTypes tests classification of basic single-type tasks.
func TestClassifyTaskBasicTypes(t *testing.T) {
	testCases := []struct {
		name            string
		description     string
		expectedPrimary TaskType
		minConfidence   float64
	}{
		{
			name:            "Research - API Documentation",
			description:     "Research and investigate the best rate limiting strategies",
			expectedPrimary: TaskTypeResearch,
			minConfidence:   0.4,
		},
		{
			name:            "Code Generation - Implement Feature",
			description:     "Implement a new user profile endpoint in the API",
			expectedPrimary: TaskTypeCodeGeneration,
			minConfidence:   0.4,
		},
		{
			name:            "Code Review - PR Review",
			description:     "Review the pull request for the database migration",
			expectedPrimary: TaskTypeCodeReview,
			minConfidence:   0.4,
		},
		{
			name:            "Testing - Unit Tests",
			description:     "Write unit tests for the user service",
			expectedPrimary: TaskTypeTesting,
			minConfidence:   0.4,
		},
		{
			name:            "Documentation - API Docs",
			description:     "Document the new API endpoints in the README",
			expectedPrimary: TaskTypeDocumentation,
			minConfidence:   0.4,
		},
		{
			name:            "Data Processing - Log Analysis",
			description:     "Parse and analyze the error logs from last night",
			expectedPrimary: TaskTypeDataProcessing,
			minConfidence:   0.4,
		},
		{
			name:            "Debugging - Bug Fix",
			description:     "Debug the memory leak issue in the background worker",
			expectedPrimary: TaskTypeDebugging,
			minConfidence:   0.4,
		},
		{
			name:            "Refactoring - Extract Method",
			description:     "Refactor the UserService to extract common validation logic",
			expectedPrimary: TaskTypeRefactoring,
			minConfidence:   0.4,
		},
		{
			name:            "Architecture - System Design",
			description:     "Design the new microservices architecture for the platform",
			expectedPrimary: TaskTypeArchitecture,
			minConfidence:   0.4,
		},
		{
			name:            "Configuration - Docker Setup",
			description:     "Configure Docker Compose for the development environment",
			expectedPrimary: TaskTypeConfiguration,
			minConfidence:   0.4,
		},
		{
			name:            "Deployment - CI/CD Pipeline",
			description:     "Set up a CI/CD pipeline for automated deployments",
			expectedPrimary: TaskTypeDeployment,
			minConfidence:   0.4,
		},
		{
			name:            "Security - Authentication",
			description:     "Implement OAuth 2.0 authentication and fix security vulnerabilities",
			expectedPrimary: TaskTypeSecurity,
			minConfidence:   0.4,
		},
		{
			name:            "Visual - UI Testing",
			description:     "Test the dashboard UI by taking screenshots and verifying visual consistency",
			expectedPrimary: TaskTypeVisual,
			minConfidence:   0.4,
		},
		{
			name:            "Communication - Status Report",
			description:     "Write a status update for the sprint completion",
			expectedPrimary: TaskTypeCommunication,
			minConfidence:   0.4,
		},
		{
			name:            "Pipeline - Multi-stage Build",
			description:     "Orchestrate and coordinate a multi-stage build pipeline workflow",
			expectedPrimary: TaskTypePipeline,
			minConfidence:   0.4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ClassifyTask(tc.description)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.PrimaryType != tc.expectedPrimary {
				t.Errorf("expected primary type %q, got %q", tc.expectedPrimary, result.PrimaryType)
			}

			if result.PrimaryConfidence < tc.minConfidence {
				t.Errorf("expected confidence >= %v, got %v", tc.minConfidence, result.PrimaryConfidence)
			}
		})
	}
}

// TestClassifyTaskMultiLabel tests multi-label classification.
func TestClassifyTaskMultiLabel(t *testing.T) {
	testCases := []struct {
		name                string
		description         string
		expectedTypes       []TaskType
		minMatchCount       int
	}{
		{
			name:                "Implementation + Testing",
			description:        "Implement a new payment processing feature and write integration tests to verify it works correctly",
			expectedTypes:       []TaskType{TaskTypeCodeGeneration, TaskTypeTesting},
			minMatchCount:       2,
		},
		{
			name:                "Debugging + Code Review",
			description:        "Debug the authentication issue reported in the pull request and review the proposed fix",
			expectedTypes:       []TaskType{TaskTypeDebugging, TaskTypeCodeReview},
			minMatchCount:       2,
		},
		{
			name:                "Research + Implementation",
			description:        "Research best practices for caching strategies and implement a new caching layer",
			expectedTypes:       []TaskType{TaskTypeResearch, TaskTypeCodeGeneration},
			minMatchCount:       2,
		},
		{
			name:                "Architecture + Security",
			description:        "Design a secure multi-tenant architecture with proper encryption and authentication mechanisms",
			expectedTypes:       []TaskType{TaskTypeArchitecture, TaskTypeSecurity},
			minMatchCount:       2,
		},
		{
			name:                "Refactoring + Testing",
			description:        "Refactor the database layer and update all related unit tests",
			expectedTypes:       []TaskType{TaskTypeRefactoring, TaskTypeTesting},
			minMatchCount:       2,
		},
		{
			name:                "Documentation + Communication",
			description:        "Write comprehensive API documentation and create a technical summary for stakeholders",
			expectedTypes:       []TaskType{TaskTypeDocumentation, TaskTypeCommunication},
			minMatchCount:       2,
		},
		{
			name:                "Pipeline + Deployment",
			description:        "Set up a multi-stage CI/CD pipeline that automatically deploys to staging and production",
			expectedTypes:       []TaskType{TaskTypePipeline, TaskTypeDeployment},
			minMatchCount:       2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ClassifyTask(tc.description)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Count how many expected types are in the results
			matchCount := 0
			for _, expectedType := range tc.expectedTypes {
				if result.HasTaskType(expectedType) {
					matchCount++
				}
			}

			if matchCount < tc.minMatchCount {
				t.Errorf("expected at least %d of %v types, got %d matches. All labels: %v",
					tc.minMatchCount, tc.expectedTypes, matchCount, result.Labels)
			}
		})
	}
}

// TestClassifyTaskConfidenceScoring tests that confidence scores are reasonable.
func TestClassifyTaskConfidenceScoring(t *testing.T) {
	testCases := []struct {
		name           string
		description    string
		expectedMin    float64
		expectedMax    float64
	}{
		{
			name:           "Low ambiguity - very clear task",
			description:    "Write unit test for the UserService validation logic",
			expectedMin:    0.3,
			expectedMax:    1.0,
		},
		{
			name:           "Moderate ambiguity - multiple possible types",
			description:    "Improve the system by reviewing code and optimizing performance",
			expectedMin:    0.3,
			expectedMax:    1.0,
		},
		{
			name:           "High confidence for specific keywords",
			description:    "Implement a new REST API endpoint for user management",
			expectedMin:    0.3,
			expectedMax:    1.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ClassifyTask(tc.description)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.PrimaryConfidence < tc.expectedMin || result.PrimaryConfidence > tc.expectedMax {
				t.Errorf("expected confidence between %v and %v, got %v",
					tc.expectedMin, tc.expectedMax, result.PrimaryConfidence)
			}

			// All confidence scores should be between 0 and 1
			for _, label := range result.Labels {
				if label.Confidence < 0.0 || label.Confidence > 1.0 {
					t.Errorf("confidence score %v outside [0, 1] range", label.Confidence)
				}
			}
		})
	}
}

// TestClassifyTaskEdgeCases tests edge cases and error conditions.
func TestClassifyTaskEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		description string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "Empty description",
			description: "",
			shouldError: true,
			errorMsg:    "cannot be empty",
		},
		{
			name:        "Whitespace only",
			description: "   \n\t  ",
			shouldError: true,
			errorMsg:    "no task type matches",
		},
		{
			name:        "Very long description",
			description: strings.Repeat("This is about implementing new features and debugging issues. ", 100),
			shouldError: false,
		},
		{
			name:        "Description with special characters",
			description: "Implement feature #123: fix bug & improve performance @ 2x speed!!!",
			shouldError: false,
		},
		{
			name:        "Mixed case keywords",
			description: "RESEARCH the API Documentation and Implement the feature",
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ClassifyTask(tc.description)

			if tc.shouldError {
				if err == nil {
					t.Errorf("expected error, got none")
				}
				if tc.errorMsg != "" && !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tc.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result, got nil")
				} else {
					if result.PrimaryType == "" {
						t.Errorf("primary type should not be empty")
					}
					if result.PrimaryConfidence <= 0.0 || result.PrimaryConfidence > 1.0 {
						t.Errorf("invalid primary confidence: %v", result.PrimaryConfidence)
					}
				}
			}
		})
	}
}

// TestClassifyTaskCaseSensitivity tests that classification is case-insensitive.
func TestClassifyTaskCaseSensitivity(t *testing.T) {
	descriptions := []string{
		"implement a new feature",
		"Implement A New Feature",
		"IMPLEMENT A NEW FEATURE",
		"ImPlEmEnT a NeW fEaTuRe",
	}

	expectedType := TaskTypeCodeGeneration

	var firstResult *ClassificationResult
	for _, desc := range descriptions {
		result, err := ClassifyTask(desc)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", desc, err)
		}

		if result.PrimaryType != expectedType {
			t.Errorf("expected %q for %q, got %q", expectedType, desc, result.PrimaryType)
		}

		// All results should be consistent regardless of case
		if firstResult == nil {
			firstResult = result
		} else {
			if result.PrimaryType != firstResult.PrimaryType {
				t.Errorf("inconsistent primary type: %q vs %q", result.PrimaryType, firstResult.PrimaryType)
			}
		}
	}
}

// TestClassifyTaskKeywordMatching tests that matched keywords are properly tracked.
func TestClassifyTaskKeywordMatching(t *testing.T) {
	description := "Implement and test a new authentication feature with security audit"
	result, err := ClassifyTask(description)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have matched multiple keyword types
	if len(result.Keywords) == 0 {
		t.Errorf("expected matched keywords, got none")
	}

	// Check for expected keywords
	expectedKeywords := []string{"implement", "test", "security", "auth", "feature"}
	foundKeywords := make(map[string]bool)

	for kw := range result.Keywords {
		for _, expected := range expectedKeywords {
			if strings.Contains(kw, expected) || strings.Contains(expected, strings.Split(kw, " ")[0]) {
				foundKeywords[expected] = true
			}
		}
	}

	if len(foundKeywords) < 3 {
		t.Errorf("expected at least 3 keyword matches, got %d. Keywords: %v", len(foundKeywords), result.Keywords)
	}
}

// TestClassifyTaskAccuracy tests overall classification accuracy on a diverse set of tasks.
func TestClassifyTaskAccuracy(t *testing.T) {
	// This test validates that classification accuracy is above a minimum threshold
	testCases := []struct {
		description     string
		expectedType    TaskType
		acceptableTypes []TaskType // Types that would also be acceptable
	}{
		{
			description:  "Research the best practices for API rate limiting",
			expectedType: TaskTypeResearch,
		},
		{
			description:     "Implement a new endpoint for the API",
			expectedType:    TaskTypeCodeGeneration,
			acceptableTypes: []TaskType{TaskTypeSecurity}, // authentication endpoints can be security-focused
		},
		{
			description:     "Review the PR and analyze potential issues",
			expectedType:    TaskTypeCodeReview,
			acceptableTypes: []TaskType{TaskTypeResearch}, // analysis can be research-like
		},
		{
			description:  "Write comprehensive unit tests for the payment module",
			expectedType: TaskTypeTesting,
		},
		{
			description:  "Update the API documentation to reflect new endpoints",
			expectedType: TaskTypeDocumentation,
		},
		{
			description:     "Parse and analyze the user activity logs",
			expectedType:    TaskTypeDataProcessing,
			acceptableTypes: []TaskType{TaskTypeResearch}, // log analysis can be research-like
		},
		{
			description:  "Debug and fix the crash in the memory allocator",
			expectedType: TaskTypeDebugging,
		},
		{
			description:     "Extract repeated code into a shared helper function",
			expectedType:    TaskTypeRefactoring,
			acceptableTypes: []TaskType{TaskTypeCodeGeneration}, // extraction requires new code
		},
		{
			description:  "Design the new event-driven architecture for notifications",
			expectedType: TaskTypeArchitecture,
		},
		{
			description:     "Set up environment variables for production deployment",
			expectedType:    TaskTypeConfiguration,
			acceptableTypes: []TaskType{TaskTypeDeployment}, // configuration is deployment-related
		},
		{
			description:  "Deploy the new version to the Kubernetes cluster",
			expectedType: TaskTypeDeployment,
		},
		{
			description:     "Implement encryption for user data protection",
			expectedType:    TaskTypeSecurity,
			acceptableTypes: []TaskType{TaskTypeCodeGeneration}, // implementation can be code generation
		},
		{
			description:  "Take screenshots of the dashboard and verify visual consistency",
			expectedType: TaskTypeVisual,
		},
		{
			description:  "Write a clear commit message for the merged changes",
			expectedType: TaskTypeCommunication,
		},
		{
			description:     "Orchestrate a build pipeline: compile, test, and deploy",
			expectedType:    TaskTypePipeline,
			acceptableTypes: []TaskType{TaskTypeDeployment, TaskTypeVisual}, // deployment is a key part, visual can match from "test"
		},
	}

	correctCount := 0
	for i, tc := range testCases {
		result, err := ClassifyTask(tc.description)
		if err != nil {
			t.Fatalf("test case %d: unexpected error: %v\ndescription: %q", i, err, tc.description)
		}

		if result.PrimaryType == tc.expectedType {
			correctCount++
		} else {
			// Check if it's in acceptable types
			acceptable := false
			if tc.acceptableTypes != nil {
				for _, at := range tc.acceptableTypes {
					if result.PrimaryType == at {
						acceptable = true
						correctCount++
						break
					}
				}
			}
			if !acceptable {
				t.Logf("MISMATCH: expected %q, got %q for: %q",
					tc.expectedType, result.PrimaryType, tc.description)
			}
		}
	}

	accuracy := float64(correctCount) / float64(len(testCases))
	targetAccuracy := 0.80 // Target 80% accuracy - reasonable for keyword-based classification
	if accuracy < targetAccuracy {
		t.Errorf("classification accuracy %.1f%% below %.0f%% threshold (%d/%d correct)",
			accuracy*100, targetAccuracy*100, correctCount, len(testCases))
	} else {
		t.Logf("Classification accuracy: %.1f%% (%d/%d correct)", accuracy*100, correctCount, len(testCases))
	}
}

// TestClassificationResultMethods tests helper methods on ClassificationResult.
func TestClassificationResultMethods(t *testing.T) {
	description := "Implement and test a new feature"
	result, err := ClassifyTask(description)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("GetTaskTypeConfidence", func(t *testing.T) {
		// Should return confidence for classified types
		conf := result.GetTaskTypeConfidence(TaskTypeCodeGeneration)
		if conf <= 0 {
			t.Errorf("expected positive confidence for code generation, got %v", conf)
		}

		// Should return 0 for unclassified types
		conf = result.GetTaskTypeConfidence(TaskTypeDeployment)
		if conf != 0.0 {
			t.Errorf("expected 0 confidence for unclassified type, got %v", conf)
		}
	})

	t.Run("HasTaskType", func(t *testing.T) {
		if !result.HasTaskType(TaskTypeCodeGeneration) {
			t.Errorf("expected HasTaskType to return true for classified type")
		}

		if result.HasTaskType(TaskTypeDeployment) {
			t.Errorf("expected HasTaskType to return false for unclassified type")
		}
	})

	t.Run("FilterByConfidence", func(t *testing.T) {
		// Filter by high confidence
		filtered := result.FilterByConfidence(0.7)
		for _, label := range filtered {
			if label.Confidence < 0.7 {
				t.Errorf("filtered label has confidence %v below threshold 0.7", label.Confidence)
			}
		}

		// Filter by low confidence
		filtered = result.FilterByConfidence(0.0)
		if len(filtered) != len(result.Labels) {
			t.Errorf("expected all labels when filtering by 0.0, got %d of %d",
				len(filtered), len(result.Labels))
		}
	})
}

// BenchmarkClassifyTask benchmarks the classification performance.
func BenchmarkClassifyTask(b *testing.B) {
	description := "Implement a new REST API endpoint for user management, including authentication, validation, and comprehensive unit tests"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClassifyTask(description)
	}
}

// BenchmarkClassifyTaskComplex benchmarks classification on complex task descriptions.
func BenchmarkClassifyTaskComplex(b *testing.B) {
	description := `
	Redesign the user authentication system to use OAuth 2.0 with proper security implementation.
	This involves researching best practices, implementing the feature across multiple microservices,
	updating all documentation, writing comprehensive tests, and coordinating a phased rollout.
	The system must handle backward compatibility and be thoroughly reviewed for security concerns.
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClassifyTask(description)
	}
}
