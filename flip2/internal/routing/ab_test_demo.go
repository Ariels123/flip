package routing

// Demo file showing how to use A/B testing for routing
// This file demonstrates the API without executing (for documentation purposes)

/*

Example 1: Enable A/B Testing on RulesEngine
==============================================

	engine := NewRulesEngine()

	// Set up A/B test: 30% of tasks go to Haiku, 70% to Sonnet
	config := &ABTestConfig{
		Percentage:   30,
		VariantModel: ModelHaiku,
		ControlModel: ModelSonnet,
	}

	if err := engine.EnableABTest(config); err != nil {
		// Handle error
	}

	// Now RouteTask will randomly assign 30% to Haiku, rest to Sonnet
	model := engine.RouteTask("task-123", TaskTypeCodeGeneration, 2.5)
	// model will be either ModelHaiku or ModelSonnet based on random assignment

	// Disable when done
	engine.DisableABTest()


Example 2: Create and Manage ABTest Experiment
===============================================

	// Create a standalone A/B test experiment
	abtest, err := NewABTest("exp-001", ModelSonnet, ModelHaiku, 25)
	if err != nil {
		// Handle error
	}

	// Record outcomes from tasks routed through experiment
	err = abtest.RecordOutcome("task-1", "control", ModelSonnet, 0.05, true, 1200)
	err = abtest.RecordOutcome("task-2", "variant", ModelHaiku, 0.02, true, 800)
	err = abtest.RecordOutcome("task-3", "control", ModelSonnet, 0.06, false, 1500)

	// Generate comprehensive report comparing performance
	report := abtest.GenerateABReport()
	// Report includes:
	// - Experiment info (control/variant models, percentage split)
	// - Summary comparison (task counts, success rates, costs, durations)
	// - Detailed breakdown for each variant
	// - Cost efficiency analysis with recommendations


Example 3: Analyze ABTest Results
==================================

	// Get outcomes for a specific variant
	variantOutcomes := abtest.FilterOutcomesByVariant("variant")
	variantOutcomes.SortByCost()

	// Export all outcomes for further analysis
	allOutcomes := abtest.ExportOutcomes()

	// Check experiment status
	if abtest.Active {
		// Still running
	}

	// Stop the experiment
	abtest.Stop()


Key Features:
=============

1. A/B Routing (rules.go):
   - ABTestConfig struct for configuration
   - EnableABTest() method to activate A/B routing in RulesEngine
   - DisableABTest() method to turn off A/B routing
   - RouteTask() automatically applies A/B split when enabled
   - All routing decisions respect override precedence

2. A/B Experiment Management (ab_test.go):
   - NewABTest() creates experiments with validation
   - RecordOutcome() logs task results with full error checking
   - GenerateABReport() produces detailed comparison reports
   - Thread-safe access via sync.RWMutex
   - Export and filter outcomes for analysis

3. Comprehensive Logging:
   - ABTestOutcome captures: TaskID, Variant, Model, Cost, Success, Duration, Timestamp
   - All outcomes stored in sequence for time-series analysis
   - Metrics calculated on-demand: counts, success rates, costs, durations

4. Cost & Performance Analysis:
   - Average cost per task comparison
   - Total cost analysis across variants
   - Success rate tracking
   - Duration metrics (min, max, average)
   - Cost per successful task calculation

*/
