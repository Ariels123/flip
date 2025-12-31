package commmonitor

import (
	"testing"
)

func TestFuzzyMatchAgent_ExactMatch(t *testing.T) {
	monitor := &Monitor{
		config: Config{Threshold: 0.75},
	}

	// Note: Claud-win uses Capital C per Windows sync (SYNC-STATUS-1766055837)
	tests := []struct {
		input    string
		expected string
	}{
		{"Claud-win", "Claud-win"},
		{"claude-mac", "claude-mac"},
		{"ag-win", "ag-win"},
		{"antigravity", "antigravity"},
		{"gemini", "gemini"},
		{"comm-monitor", "comm-monitor"},
	}

	for _, tt := range tests {
		result := monitor.fuzzyMatchAgent(tt.input)
		if result != tt.expected {
			t.Errorf("fuzzyMatchAgent(%q) = %q; want %q",
				tt.input, result, tt.expected)
		}
	}
}

func TestFuzzyMatchAgent_CaseInsensitive(t *testing.T) {
	monitor := &Monitor{
		config: Config{Threshold: 0.75},
	}

	// Note: Claud-win uses Capital C per Windows sync (SYNC-STATUS-1766055837)
	tests := []struct {
		input    string
		expected string
	}{
		{"CLAUD-WIN", "Claud-win"},
		{"Claud-Win", "Claud-win"},
		{"CLAUDE-MAC", "claude-mac"},
		{"Claude-Mac", "claude-mac"},
		{"AG-WIN", "ag-win"},
		{"ANTIGRAVITY", "antigravity"},
		{"GEMINI", "gemini"},
	}

	for _, tt := range tests {
		result := monitor.fuzzyMatchAgent(tt.input)
		if result != tt.expected {
			t.Errorf("fuzzyMatchAgent(%q) = %q; want %q",
				tt.input, result, tt.expected)
		}
	}
}

func TestFuzzyMatchAgent_TypoCorrections(t *testing.T) {
	monitor := &Monitor{
		config: Config{Threshold: 0.75},
	}

	// Note: Claud-win uses Capital C per Windows sync (SYNC-STATUS-1766055837)
	tests := []struct {
		input    string
		expected string
	}{
		{"claude-win", "Claud-win"},  // Common typo
		{"claudwin", "Claud-win"},
		{"claud win", "Claud-win"},
		{"calude-win", "Claud-win"},
		{"claud-win", "Claud-win"},   // lowercase corrected to Capital C

		{"claude mac", "claude-mac"},
		{"claudemac", "claude-mac"},

		{"agwin", "ag-win"},
		{"ag win", "ag-win"},

		{"anti-gravity", "antigravity"},
		{"anti gravity", "antigravity"},
	}

	for _, tt := range tests {
		result := monitor.fuzzyMatchAgent(tt.input)
		if result != tt.expected {
			t.Errorf("fuzzyMatchAgent(%q) = %q; want %q",
				tt.input, result, tt.expected)
		}
	}
}

func TestFuzzyMatchAgent_FuzzyMatch(t *testing.T) {
	monitor := &Monitor{
		config: Config{Threshold: 0.75},
	}

	// Note: Claud-win uses Capital C per Windows sync (SYNC-STATUS-1766055837)
	tests := []struct {
		input    string
		expected string
	}{
		{"claud-wn", "Claud-win"},    // Missing 'i' - corrected to Capital C
		{"claude-ma", "claude-mac"},  // Missing 'c'
		{"gemin", "gemini"},          // Missing 'i'
		{"ag-wn", "ag-win"},          // Missing 'i'
	}

	for _, tt := range tests {
		result := monitor.fuzzyMatchAgent(tt.input)
		if result != tt.expected {
			t.Errorf("fuzzyMatchAgent(%q) = %q; want %q",
				tt.input, result, tt.expected)
		}
	}
}

func TestFuzzyMatchAgent_NoMatch(t *testing.T) {
	monitor := &Monitor{
		config: Config{Threshold: 0.75},
	}

	tests := []string{
		"completely-unknown",
		"random-agent",
		"xyz",
		"",
	}

	for _, input := range tests {
		result := monitor.fuzzyMatchAgent(input)
		if result != "" {
			t.Errorf("fuzzyMatchAgent(%q) = %q; want empty string",
				input, result)
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "adc", 1},
		{"abc", "axc", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
	}

	for _, tt := range tests {
		result := levenshteinDistance(tt.s1, tt.s2)
		if result != tt.expected {
			t.Errorf("levenshteinDistance(%q, %q) = %d; want %d",
				tt.s1, tt.s2, result, tt.expected)
		}
	}
}

func TestCalculateSimilarity(t *testing.T) {
	monitor := &Monitor{}

	tests := []struct {
		s1       string
		s2       string
		minSim   float64
		maxSim   float64
	}{
		{"abc", "abc", 1.0, 1.0},
		{"abc", "abd", 0.6, 0.7},
		{"claud-win", "claude-win", 0.8, 1.0},
		{"gemini", "gemin", 0.8, 0.9},
		{"xyz", "abc", 0.0, 0.1},
	}

	for _, tt := range tests {
		result := monitor.calculateSimilarity(tt.s1, tt.s2)
		if result < tt.minSim || result > tt.maxSim {
			t.Errorf("calculateSimilarity(%q, %q) = %f; want between %f and %f",
				tt.s1, tt.s2, result, tt.minSim, tt.maxSim)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.PollInterval == 0 {
		t.Error("DefaultConfig PollInterval should not be 0")
	}

	if config.Threshold <= 0 || config.Threshold > 1 {
		t.Errorf("DefaultConfig Threshold should be between 0 and 1, got %f", config.Threshold)
	}

	if !config.Enabled {
		t.Error("DefaultConfig should be Enabled by default")
	}
}

func TestValidAgents(t *testing.T) {
	// Note: Claud-win uses Capital C per Windows sync (SYNC-STATUS-1766055837)
	expectedAgents := []string{
		"Claud-win",
		"claude-mac",
		"ag-win",
		"antigravity",
		"gemini",
		"comm-monitor",
		"claude",
		"cli",
	}

	for _, agent := range expectedAgents {
		if !ValidAgents[agent] {
			t.Errorf("Expected %q to be in ValidAgents", agent)
		}
	}
}

func TestTypoCorrections(t *testing.T) {
	// Verify all typo corrections map to valid agents
	for typo, corrected := range TypoCorrections {
		if !ValidAgents[corrected] {
			t.Errorf("TypoCorrections[%q] = %q, but %q is not a valid agent",
				typo, corrected, corrected)
		}
	}
}

func BenchmarkFuzzyMatchAgent(b *testing.B) {
	monitor := &Monitor{
		config: Config{Threshold: 0.75},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.fuzzyMatchAgent("claude-win")
	}
}

func BenchmarkLevenshteinDistance(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		levenshteinDistance("claud-win", "claude-win")
	}
}
