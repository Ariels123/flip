package main

import (
	"fmt"
	"flip2/internal/routing"
)

func main() {
	testCases := []struct {
		name        string
		description string
	}{
		{"Trivial", "Fix typo in README"},
		{"Simple", "Write unit test for string utilities"},
		{"Moderate", "Implement REST endpoint for user authentication"},
		{"Complex", "Refactor authentication system for multi-factor auth"},
		{"HighlyComplex", "Design microservices architecture for payment processing"},
	}

	for _, tc := range testCases {
		score, err := routing.ScoreTask(tc.description)
		if err != nil {
			fmt.Printf("%s: ERROR %v\n", tc.name, err)
			continue
		}

		overall := score.OverallScore()
		fmt.Printf("%s ('%s'):\n", tc.name, tc.description)
		fmt.Printf("  Technical: %d, Context: %d, Risk: %d, Reversibility: %d\n",
			score.TechnicalComplexity, score.ContextRequirements, 
			score.RiskLevel, score.Reversibility)
		fmt.Printf("  Overall: %.2f, Tokens: %d, ReviewRequired: %v\n\n",
			overall, score.EstimatedTokens, score.RequiresHumanReview)
	}
}
