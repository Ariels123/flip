package routing

import (
	"fmt"
	"testing"
)

// TestScorerAccuracy measures the overall accuracy of the complexity scorer.
// Target: >90% accuracy across all complexity levels.
func TestScorerAccuracy(t *testing.T) {
	type testCase struct {
		description string
		expectedMin float64
		expectedMax float64
		level       string
	}

	testCases := []testCase{
		// Level 1 - Trivial
		{"Rename variable x to index", 0.5, 1.8, "Trivial"},
		{"Fix typo in README", 0.5, 1.8, "Trivial"},
		{"List all files in directory", 0.5, 2.0, "Trivial"},
		{"Read configuration settings", 0.5, 1.8, "Trivial"},
		{"Add simple comment to function", 0.5, 1.8, "Trivial"},

		// Level 2 - Simple
		{"Write unit tests for string utilities", 0.8, 2.5, "Simple"},
		{"Add a simple feature to calculate factorial", 0.8, 2.5, "Simple"},
		{"Fix the bug where counter doesn't reset", 0.8, 2.5, "Simple"},
		{"Update YAML configuration for environment variable", 0.8, 2.5, "Simple"},
		{"Parse JSON file to extract user IDs", 0.8, 2.5, "Simple"},

		// Level 3 - Moderate
		{"Implement REST endpoint for user authentication with JWT", 1.5, 4.5, "Moderate"},
		{"Create database migration for new status column", 1.5, 4.5, "Moderate"},
		{"Refactor authentication module using dependency injection", 1.5, 4.5, "Moderate"},
		{"Integrate payment service API with error handling", 1.5, 4.5, "Moderate"},
		{"Add Redis caching to user lookup function", 1.0, 4.5, "Moderate"},

		// Level 4 - Complex
		{"Refactor entire authentication system for multi-factor authentication", 2.5, 5.0, "Complex"},
		{"Debug race condition in distributed cache synchronization", 2.0, 5.0, "Complex"},
		{"Security audit of authentication and authorization system", 2.0, 5.0, "Complex"},
		{"Optimize database queries and caching", 2.5, 5.0, "Complex"},
		{"Implement real-time data sync between payment and accounting", 2.0, 5.0, "Complex"},

		// Level 5 - Highly Complex
		{"Design complete microservices architecture for payment platform", 2.8, 5.0, "HighlyComplex"},
		{"Implement machine learning algorithm for resource allocation", 2.0, 5.0, "HighlyComplex"},
		{"Design end-to-end encryption with key rotation", 2.5, 5.0, "HighlyComplex"},
		{"Plan zero-downtime migration of 100M records", 1.5, 5.0, "HighlyComplex"},
		{"Design framework for distributed transactions", 2.0, 5.0, "HighlyComplex"},
	}

	var correct, total int
	levelCorrect := make(map[string]int)
	levelTotal := make(map[string]int)
	var results string

	results += "COMPLEXITY SCORER ACCURACY ANALYSIS\n"
	results += "====================================\n\n"

	for _, tc := range testCases {
		score, err := ScoreTask(tc.description)
		if err != nil {
			t.Errorf("Error scoring task '%s': %v", tc.description, err)
			continue
		}

		overall := score.OverallScore()
		isCorrect := overall >= tc.expectedMin && overall <= tc.expectedMax

		total++
		levelTotal[tc.level]++
		if isCorrect {
			correct++
			levelCorrect[tc.level]++
		}

		status := "✓"
		if !isCorrect {
			status = "✗"
		}

		results += fmt.Sprintf("%s [%s] %s\n", status, tc.level, tc.description)
		results += fmt.Sprintf("   Score: %.2f (expected %.1f-%.1f)\n", overall, tc.expectedMin, tc.expectedMax)
		results += fmt.Sprintf("   Components: Tech=%d, Context=%d, Risk=%d, Rev=%d\n\n",
			score.TechnicalComplexity, score.ContextRequirements, score.RiskLevel, score.Reversibility)
	}

	accuracy := float64(correct) / float64(total) * 100

	results += "\nACCURACY SUMMARY\n"
	results += "================\n"
	results += fmt.Sprintf("Overall Accuracy: %d/%d (%.1f%%)\n", correct, total, accuracy)
	results += "\nAccuracy by Level:\n"

	for _, level := range []string{"Trivial", "Simple", "Moderate", "Complex", "HighlyComplex"} {
		if levelTotal[level] > 0 {
			acc := float64(levelCorrect[level]) / float64(levelTotal[level]) * 100
			results += fmt.Sprintf("  %s: %d/%d (%.1f%%)\n", level, levelCorrect[level], levelTotal[level], acc)
		}
	}

	if accuracy >= 90.0 {
		results += "\n✓ ACCURACY TARGET ACHIEVED (>=90%)\n"
	} else {
		results += fmt.Sprintf("\n✗ Accuracy below target: %.1f%% (target: >=90%%)\n", accuracy)
	}

	// Log results
	t.Log(results)

	// Verify accuracy meets target
	if accuracy < 90.0 {
		t.Errorf("Scorer accuracy %.1f%% is below target of 90%%", accuracy)
	}
}
