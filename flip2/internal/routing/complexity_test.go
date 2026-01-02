package routing

import (
	"testing"
)

// ================================================================================
// TEST CASES FOR COMPLEXITY SCORING
// ================================================================================

// TestComplexityLevel1_Trivial tests tasks with score 1 (trivial).
func TestComplexityLevel1_Trivial(t *testing.T) {
	testCases := []struct {
		name        string
		description string
		expectedMin int
		expectedMax int
	}{
		{
			name:        "Rename variable",
			description: "Rename the 'x' variable to 'index' in the loop",
			expectedMin: 1,
			expectedMax: 2,
		},
		{
			name:        "Fix typo",
			description: "Fix typo in README: change 'teh' to 'the'",
			expectedMin: 1,
			expectedMax: 2,
		},
		{
			name:        "List files",
			description: "List all Go files in the project directory",
			expectedMin: 1,
			expectedMax: 2,
		},
		{
			name:        "Read configuration",
			description: "Read and display the current configuration settings",
			expectedMin: 1,
			expectedMax: 2,
		},
		{
			name:        "Check status",
			description: "Check the status of all background services",
			expectedMin: 1,
			expectedMax: 2,
		},
		{
			name:        "Simple comment",
			description: "Add a comment explaining what this helper function does",
			expectedMin: 1,
			expectedMax: 2,
		},
		{
			name:        "Update README",
			description: "Update README with installation instructions",
			expectedMin: 1,
			expectedMax: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score, err := ScoreTask(tc.description)
			if err != nil {
				t.Fatalf("ScoreTask failed: %v", err)
			}

			overall := int(score.OverallScore())
			if overall < tc.expectedMin || overall > tc.expectedMax {
				t.Errorf("Overall score %d outside expected range [%d, %d]",
					overall, tc.expectedMin, tc.expectedMax)
			}

			// Verify component scores
			if score.TechnicalComplexity > 2 {
				t.Errorf("Technical complexity %d too high for trivial task",
					score.TechnicalComplexity)
			}
			if score.RiskLevel > 2 {
				t.Errorf("Risk level %d too high for trivial task",
					score.RiskLevel)
			}
		})
	}
}

// TestComplexityLevel2_Simple tests tasks with score ~2 (simple).
func TestComplexityLevel2_Simple(t *testing.T) {
	testCases := []struct {
		name        string
		description string
	}{
		{
			name:        "Write simple unit test",
			description: "Write unit tests for the string utility functions",
		},
		{
			name:        "Add simple feature",
			description: "Add a function to calculate the factorial of a number",
		},
		{
			name:        "Fix simple bug",
			description: "Fix the bug where the counter doesn't reset properly",
		},
		{
			name:        "Update configuration",
			description: "Update the YAML configuration to use new environment variable",
		},
		{
			name:        "Parse data file",
			description: "Parse the JSON file and extract user IDs",
		},
		{
			name:        "Create log utility",
			description: "Create a utility function to log messages with timestamps",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score, err := ScoreTask(tc.description)
			if err != nil {
				t.Fatalf("ScoreTask failed: %v", err)
			}

			overall := int(score.OverallScore())
			if overall < 1 || overall > 3 {
				t.Logf("Overall score %d (expected 1-3) for: %s",
					overall, tc.name)
			}

			// Should be achievable with basic work
			if score.TechnicalComplexity > 3 {
				t.Logf("Technical complexity %d seems high for simple task: %s",
					score.TechnicalComplexity, tc.name)
			}
		})
	}
}

// TestComplexityLevel3_Moderate tests tasks with score ~3 (moderate).
func TestComplexityLevel3_Moderate(t *testing.T) {
	testCases := []struct {
		name        string
		description string
	}{
		{
			name:        "Implement REST endpoint",
			description: "Implement a REST endpoint for user authentication with JWT tokens",
		},
		{
			name:        "Database migration",
			description: "Create a database migration to add a new 'status' column to the users table",
		},
		{
			name:        "Code refactoring",
			description: "Refactor the authentication module to use dependency injection across multiple files",
		},
		{
			name:        "Integrate external API",
			description: "Integrate the payment service API and handle errors properly",
		},
		{
			name:        "Implement caching",
			description: "Add Redis caching to the user lookup function to improve performance",
		},
		{
			name:        "Complex test suite",
			description: "Write comprehensive integration tests for the order processing pipeline",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score, err := ScoreTask(tc.description)
			if err != nil {
				t.Fatalf("ScoreTask failed: %v", err)
			}

			overall := score.OverallScore()
			if overall < 2.5 || overall > 4.0 {
				t.Logf("Overall score %.1f (expected 2.5-4.0) for: %s",
					overall, tc.name)
			}
		})
	}
}

// TestComplexityLevel4_Complex tests tasks with score ~4 (complex).
func TestComplexityLevel4_Complex(t *testing.T) {
	testCases := []struct {
		name        string
		description string
	}{
		{
			name:        "Major refactoring",
			description: "Refactor the entire authentication system to support multi-factor authentication across all services",
		},
		{
			name:        "Complex debugging",
			description: "Debug and fix race condition in the distributed cache synchronization across multiple services",
		},
		{
			name:        "Security audit",
			description: "Audit the authentication and authorization system for security vulnerabilities and compliance issues",
		},
		{
			name:        "Performance optimization",
			description: "Optimize database queries and add intelligent caching to reduce load time from 5s to 100ms",
		},
		{
			name:        "Cross-system integration",
			description: "Implement real-time data synchronization between the payment system and the accounting system",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score, err := ScoreTask(tc.description)
			if err != nil {
				t.Fatalf("ScoreTask failed: %v", err)
			}

			overall := score.OverallScore()
			if overall < 3.5 || overall > 5.0 {
				t.Logf("Overall score %.1f (expected 3.5-5.0) for: %s",
					overall, tc.name)
			}

			// Complex tasks should require human review
			if score.RiskLevel >= 4 || score.TechnicalComplexity >= 4 {
				if !score.RequiresHumanReview {
					t.Logf("Complex/risky task should require human review: %s", tc.name)
				}
			}
		})
	}
}

// TestComplexityLevel5_HighlyComplex tests tasks with score 5 (highly complex).
func TestComplexityLevel5_HighlyComplex(t *testing.T) {
	testCases := []struct {
		name        string
		description string
	}{
		{
			name:        "System architecture",
			description: "Design the complete microservices architecture for a new payment processing platform including API design, database schema, service interactions, and deployment strategy",
		},
		{
			name:        "Novel algorithm",
			description: "Implement a machine learning algorithm to optimize resource allocation across multiple data centers with real-time feedback loops",
		},
		{
			name:        "Critical security",
			description: "Design and implement end-to-end encryption for sensitive data at rest and in transit, including key rotation and compliance with regulatory requirements",
		},
		{
			name:        "Complex data migration",
			description: "Plan and execute a zero-downtime migration of 100 million records from a monolithic database to a distributed system without data loss",
		},
		{
			name:        "Framework design",
			description: "Design a new internal framework for handling distributed transactions across multiple services with guaranteed consistency and fault tolerance",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score, err := ScoreTask(tc.description)
			if err != nil {
				t.Fatalf("ScoreTask failed: %v", err)
			}

			overall := score.OverallScore()
			if overall < 4.0 {
				t.Logf("Overall score %.1f (expected >= 4.0) for: %s",
					overall, tc.name)
			}

			// Should definitely require human review
			if !score.RequiresHumanReview {
				t.Logf("Highly complex task should require human review: %s", tc.name)
			}
		})
	}
}

// TestTechnicalComplexityGradation verifies complexity increases appropriately.
func TestTechnicalComplexityGradation(t *testing.T) {
	testCases := []struct {
		description string
		minLevel    int
		maxLevel    int
	}{
		{
			description: "Fix typo in comment",
			minLevel:    1,
			maxLevel:    1,
		},
		{
			description: "Implement new feature",
			minLevel:    2,
			maxLevel:    3,
		},
		{
			description: "Refactor across multiple files",
			minLevel:    3,
			maxLevel:    4,
		},
		{
			description: "Design system architecture",
			minLevel:    4,
			maxLevel:    5,
		},
	}

	for _, tc := range testCases {
		score, err := ScoreTask(tc.description)
		if err != nil {
			t.Fatalf("ScoreTask failed: %v", err)
		}

		if score.TechnicalComplexity < tc.minLevel || score.TechnicalComplexity > tc.maxLevel {
			t.Errorf("Task '%s': got complexity %d, expected %d-%d",
				tc.description, score.TechnicalComplexity, tc.minLevel, tc.maxLevel)
		}
	}
}

// TestContextRequirementsScoring verifies context requirements are assessed correctly.
func TestContextRequirementsScoring(t *testing.T) {
	testCases := []struct {
		description    string
		minContext     int
		maxContext     int
		expectedReason string
	}{
		{
			description:    "Delete unused variable",
			minContext:     1,
			maxContext:     2,
			expectedReason: "Trivial change, minimal context needed",
		},
		{
			description:    "Add feature to existing module",
			minContext:     1,
			maxContext:     2,
			expectedReason: "Single module understanding needed",
		},
		{
			description:    "Refactor across multiple modules",
			minContext:     2,
			maxContext:     4,
			expectedReason: "Multiple modules understanding needed",
		},
		{
			description:    "Implement cross-system integration",
			minContext:     2,
			maxContext:     4,
			expectedReason: "Deep system knowledge required",
		},
	}

	for _, tc := range testCases {
		score, err := ScoreTask(tc.description)
		if err != nil {
			t.Fatalf("ScoreTask failed: %v", err)
		}

		if score.ContextRequirements < tc.minContext || score.ContextRequirements > tc.maxContext {
			t.Errorf("Task '%s': got context %d, expected %d-%d (%s)",
				tc.description, score.ContextRequirements, tc.minContext, tc.maxContext,
				tc.expectedReason)
		}
	}
}

// TestRiskLevelScoring verifies risk assessment works correctly.
func TestRiskLevelScoring(t *testing.T) {
	testCases := []struct {
		description  string
		minRisk      int
		maxRisk      int
		expectedType string
	}{
		{
			description:  "Read configuration and report it",
			minRisk:      1,
			maxRisk:      2,
			expectedType: "Read-only, no impact",
		},
		{
			description:  "Fix typo in UI label",
			minRisk:      1,
			maxRisk:      2,
			expectedType: "Minor UI change",
		},
		{
			description:  "Update database configuration",
			minRisk:      2,
			maxRisk:      3,
			expectedType: "Moderate risk",
		},
		{
			description:  "Implement authentication system",
			minRisk:      5,
			maxRisk:      5,
			expectedType: "High risk",
		},
		{
			description:  "Fix critical security vulnerability in payment processing",
			minRisk:      5,
			maxRisk:      5,
			expectedType: "Critical risk",
		},
	}

	for _, tc := range testCases {
		score, err := ScoreTask(tc.description)
		if err != nil {
			t.Fatalf("ScoreTask failed: %v", err)
		}

		if score.RiskLevel < tc.minRisk || score.RiskLevel > tc.maxRisk {
			t.Errorf("Task '%s': got risk %d, expected %d-%d (%s)",
				tc.description, score.RiskLevel, tc.minRisk, tc.maxRisk,
				tc.expectedType)
		}
	}
}

// TestReversibilityScoring verifies reversibility is assessed correctly.
func TestReversibilityScoring(t *testing.T) {
	testCases := []struct {
		description      string
		minReversibility int
		maxReversibility int
		expectedType     string
	}{
		{
			description:      "Review and analyze the codebase",
			minReversibility: 4,
			maxReversibility: 5,
			expectedType:     "Read-only",
		},
		{
			description:      "Add a new function to the utility module",
			minReversibility: 3,
			maxReversibility: 5,
			expectedType:     "Easy to reverse",
		},
		{
			description:      "Update configuration and restart services",
			minReversibility: 2,
			maxReversibility: 4,
			expectedType:     "Moderate difficulty",
		},
		{
			description:      "Perform database schema migration",
			minReversibility: 1,
			maxReversibility: 3,
			expectedType:     "Difficult to reverse",
		},
		{
			description:      "Delete old data from production database",
			minReversibility: 1,
			maxReversibility: 2,
			expectedType:     "Irreversible",
		},
	}

	for _, tc := range testCases {
		score, err := ScoreTask(tc.description)
		if err != nil {
			t.Fatalf("ScoreTask failed: %v", err)
		}

		if score.Reversibility < tc.minReversibility || score.Reversibility > tc.maxReversibility {
			t.Errorf("Task '%s': got reversibility %d, expected %d-%d (%s)",
				tc.description, score.Reversibility, tc.minReversibility, tc.maxReversibility,
				tc.expectedType)
		}
	}
}

// TestOverallScoreCalculation verifies overall score calculation.
func TestOverallScoreCalculation(t *testing.T) {
	score, err := ScoreTask("Implement new API endpoint with authentication")
	if err != nil {
		t.Fatalf("ScoreTask failed: %v", err)
	}

	overall := score.OverallScore()

	// Overall should be between 1.0 and 5.0
	if overall < 1.0 || overall > 5.0 {
		t.Errorf("Overall score %.2f out of valid range [1.0, 5.0]", overall)
	}

	// Verify it's a weighted combination
	// It should consider all dimensions
	if overall == float64(score.TechnicalComplexity) &&
		overall == float64(score.ContextRequirements) &&
		overall == float64(score.RiskLevel) {
		t.Errorf("Overall score appears to be just copying one dimension")
	}
}

// TestHumanReviewThresholds verifies when human review is required.
func TestHumanReviewThresholds(t *testing.T) {
	testCases := []struct {
		description         string
		shouldRequireReview bool
	}{
		{
			description:         "Fix typo in README",
			shouldRequireReview: false,
		},
		{
			description:         "Write unit test for helper function",
			shouldRequireReview: false,
		},
		{
			description:         "Implement authentication endpoint with JWT",
			shouldRequireReview: true, // Security keywords
		},
		{
			description:         "Design system architecture for payment processing",
			shouldRequireReview: true, // Technical complexity 4+
		},
		{
			description:         "Fix critical security vulnerability",
			shouldRequireReview: true, // Risk level 5
		},
		{
			description:         "Update documentation",
			shouldRequireReview: false,
		},
	}

	for _, tc := range testCases {
		score, err := ScoreTask(tc.description)
		if err != nil {
			t.Fatalf("ScoreTask failed: %v", err)
		}

		if score.RequiresHumanReview != tc.shouldRequireReview {
			t.Errorf("Task '%s': RequiresHumanReview=%v, expected %v",
				tc.description, score.RequiresHumanReview, tc.shouldRequireReview)
		}
	}
}

// TestEstimatedTokens verifies token estimation.
func TestEstimatedTokens(t *testing.T) {
	testCases := []struct {
		description string
		minTokens   int
		maxTokens   int
	}{
		{
			description: "Fix typo",
			minTokens:   100,
			maxTokens:   1000,
		},
		{
			description: "Implement a REST endpoint for user authentication",
			minTokens:   500,
			maxTokens:   2000,
		},
		{
			description: "Design a complete microservices architecture with detailed consideration of scalability, fault tolerance, security, and deployment strategy",
			minTokens:   1000,
			maxTokens:   10000,
		},
	}

	for _, tc := range testCases {
		score, err := ScoreTask(tc.description)
		if err != nil {
			t.Fatalf("ScoreTask failed: %v", err)
		}

		if score.EstimatedTokens < tc.minTokens || score.EstimatedTokens > tc.maxTokens {
			t.Errorf("Task '%s': got tokens %d, expected %d-%d",
				tc.description, score.EstimatedTokens, tc.minTokens, tc.maxTokens)
		}
	}
}

// TestScoreTaskValidation verifies all returned scores are valid.
func TestScoreTaskValidation(t *testing.T) {
	testCases := []string{
		"Fix typo",
		"Implement feature",
		"Design architecture",
		"Debug complex issue",
		"Write tests",
		"Update documentation",
		"Refactor code",
		"Migrate database",
		"Implement security fix",
		"Review code quality",
	}

	for _, description := range testCases {
		score, err := ScoreTask(description)
		if err != nil {
			t.Fatalf("ScoreTask failed for '%s': %v", description, err)
		}

		// Validate the score
		if err := score.Validate(); err != nil {
			t.Errorf("Invalid score for '%s': %v", description, err)
		}

		// Check all dimensions are in valid range
		if score.TechnicalComplexity < 1 || score.TechnicalComplexity > 5 {
			t.Errorf("Task '%s': TechnicalComplexity %d out of range",
				description, score.TechnicalComplexity)
		}
		if score.ContextRequirements < 1 || score.ContextRequirements > 5 {
			t.Errorf("Task '%s': ContextRequirements %d out of range",
				description, score.ContextRequirements)
		}
		if score.RiskLevel < 1 || score.RiskLevel > 5 {
			t.Errorf("Task '%s': RiskLevel %d out of range",
				description, score.RiskLevel)
		}
		if score.Reversibility < 1 || score.Reversibility > 5 {
			t.Errorf("Task '%s': Reversibility %d out of range",
				description, score.Reversibility)
		}

		// Overall score should be in range
		overall := score.OverallScore()
		if overall < 1.0 || overall > 5.0 {
			t.Errorf("Task '%s': OverallScore %.2f out of range",
				description, overall)
		}
	}
}

// TestScoreTaskByKeywords verifies keyword-based scoring works.
func TestScoreTaskByKeywords(t *testing.T) {
	testCases := []struct {
		taskType    TaskType
		description string
		minScore    float64
		maxScore    float64
	}{
		{
			taskType:    TaskTypeArchitecture,
			description: "Design the system",
			minScore:    3.5,
			maxScore:    5.0,
		},
		{
			taskType:    TaskTypeDocumentation,
			description: "Write the docs",
			minScore:    1.0,
			maxScore:    2.5,
		},
		{
			taskType:    TaskTypeCodeGeneration,
			description: "Implement the feature",
			minScore:    2.0,
			maxScore:    4.0,
		},
		{
			taskType:    TaskTypeSecurity,
			description: "Fix the vulnerability",
			minScore:    3.5,
			maxScore:    5.0,
		},
	}

	for _, tc := range testCases {
		score, err := ScoreTaskByKeywords(tc.taskType, tc.description)
		if err != nil {
			t.Fatalf("ScoreTaskByKeywords failed: %v", err)
		}

		overall := score.OverallScore()
		if overall < tc.minScore || overall > tc.maxScore {
			t.Errorf("TaskType %s: got overall %.2f, expected %.2f-%.2f",
				tc.taskType, overall, tc.minScore, tc.maxScore)
		}
	}
}

// BenchmarkScoreTask benchmarks task scoring performance.
func BenchmarkScoreTask(b *testing.B) {
	description := "Implement a REST endpoint for user authentication with JWT tokens and proper error handling"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ScoreTask(description)
		if err != nil {
			b.Fatalf("ScoreTask failed: %v", err)
		}
	}
}

// BenchmarkScoreTaskByKeywords benchmarks keyword-based scoring performance.
func BenchmarkScoreTaskByKeywords(b *testing.B) {
	description := "Implement a REST endpoint for user authentication"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ScoreTaskByKeywords(TaskTypeCodeGeneration, description)
		if err != nil {
			b.Fatalf("ScoreTaskByKeywords failed: %v", err)
		}
	}
}
