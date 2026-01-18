package videoeditor

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"flip2/internal/logger"
)

func TestBatchProgress_GetStats(t *testing.T) {
	progress := NewBatchProgress()

	// Set initial values
	progress.TotalJobs = 100
	progress.CompletedJobs = 50
	progress.FailedJobs = 5
	progress.ActiveJobs = 10

	stats := progress.GetStats()

	if stats.TotalJobs != 100 {
		t.Errorf("Expected total 100, got %d", stats.TotalJobs)
	}

	if stats.CompletedJobs != 50 {
		t.Errorf("Expected completed 50, got %d", stats.CompletedJobs)
	}

	if stats.FailedJobs != 5 {
		t.Errorf("Expected failed 5, got %d", stats.FailedJobs)
	}

	if stats.ActiveJobs != 10 {
		t.Errorf("Expected active 10, got %d", stats.ActiveJobs)
	}

	expectedRate := 50.0 // 50/100 * 100
	if stats.CompletionRate != expectedRate {
		t.Errorf("Expected completion rate %.2f%%, got %.2f%%", expectedRate, stats.CompletionRate)
	}
}

func TestBatchProcessor_ProcessBatch_Empty(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	// Create mock editor
	editor, err := New(log, "ffmpeg", "ffprobe", tempDir, tempDir)
	if err != nil {
		t.Fatalf("Failed to create editor: %v", err)
	}

	tm := NewTemplateManager([]string{tempDir})
	bp := NewBatchProcessor(editor, tm, 2, log)

	// Process empty batch
	results, err := bp.ProcessBatch(context.Background(), []*BatchJob{})
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}

	stats := bp.GetProgress()
	if stats.TotalJobs != 0 {
		t.Errorf("Expected total 0, got %d", stats.TotalJobs)
	}
}

func TestBatchProcessor_ProcessBatch_TemplateNotFound(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	editor, err := New(log, "ffmpeg", "ffprobe", tempDir, tempDir)
	if err != nil {
		t.Fatalf("Failed to create editor: %v", err)
	}

	tm := NewTemplateManager([]string{tempDir})
	bp := NewBatchProcessor(editor, tm, 2, log)

	// Create job with non-existent template
	jobs := []*BatchJob{
		{
			ID:           "job1",
			TemplateName: "nonexistent",
			Variables:    map[string]interface{}{},
			OutputPath:   filepath.Join(tempDir, "output1.mp4"),
		},
	}

	results, err := bp.ProcessBatch(context.Background(), jobs)
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Success {
		t.Error("Expected job to fail")
	}

	if result.Error == "" {
		t.Error("Expected error message")
	}

	if !contains(result.Error, "template not found") && !contains(result.Error, "failed to load template") {
		t.Errorf("Expected template error, got: %s", result.Error)
	}
}

func TestGenerateBatchReport(t *testing.T) {
	results := []*BatchResult{
		{
			Job:      &BatchJob{ID: "job1"},
			Success:  true,
			Duration: 5 * time.Second,
		},
		{
			Job:      &BatchJob{ID: "job2"},
			Success:  true,
			Duration: 10 * time.Second,
		},
		{
			Job:      &BatchJob{ID: "job3"},
			Success:  false,
			Error:    "template not found",
			Duration: 1 * time.Second,
		},
		{
			Job:      &BatchJob{ID: "job4"},
			Success:  false,
			Error:    "encoding failed",
			Duration: 3 * time.Second,
		},
	}

	report := GenerateBatchReport(results)

	if report.TotalJobs != 4 {
		t.Errorf("Expected total 4, got %d", report.TotalJobs)
	}

	if report.SuccessfulJobs != 2 {
		t.Errorf("Expected successful 2, got %d", report.SuccessfulJobs)
	}

	if report.FailedJobs != 2 {
		t.Errorf("Expected failed 2, got %d", report.FailedJobs)
	}

	expectedRate := 50.0 // 2/4 * 100
	if report.SuccessRate != expectedRate {
		t.Errorf("Expected success rate %.2f%%, got %.2f%%", expectedRate, report.SuccessRate)
	}

	expectedTotalDuration := 19 * time.Second
	if report.TotalDuration != expectedTotalDuration {
		t.Errorf("Expected total duration %v, got %v", expectedTotalDuration, report.TotalDuration)
	}

	expectedAvgDuration := 4750 * time.Millisecond // 19s / 4
	if report.AverageDuration != expectedAvgDuration {
		t.Errorf("Expected avg duration %v, got %v", expectedAvgDuration, report.AverageDuration)
	}

	// Check failure reasons
	if len(report.FailureReasons) != 2 {
		t.Errorf("Expected 2 failure reasons, got %d", len(report.FailureReasons))
	}

	if report.FailureReasons["template not found"] != 1 {
		t.Errorf("Expected 1 'template not found' failure, got %d", report.FailureReasons["template not found"])
	}

	if report.FailureReasons["encoding failed"] != 1 {
		t.Errorf("Expected 1 'encoding failed' failure, got %d", report.FailureReasons["encoding failed"])
	}
}

func TestCSVDataSource_Load(t *testing.T) {
	tempDir := t.TempDir()

	// Create test CSV file
	csvContent := `id,title,subtitle,priority
video1,First Video,Subtitle 1,1
video2,Second Video,Subtitle 2,2
video3,Third Video,Subtitle 3,0`

	csvPath := filepath.Join(tempDir, "test.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("Failed to write CSV file: %v", err)
	}

	// Create data source
	ds := NewCSVDataSource(csvPath, "test-template", tempDir)

	// Load jobs
	jobs, err := ds.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(jobs) != 3 {
		t.Fatalf("Expected 3 jobs, got %d", len(jobs))
	}

	// Verify first job
	job := jobs[0]
	if job.ID != "video1" {
		t.Errorf("Expected ID 'video1', got '%s'", job.ID)
	}

	if job.TemplateName != "test-template" {
		t.Errorf("Expected template 'test-template', got '%s'", job.TemplateName)
	}

	if job.Variables["title"] != "First Video" {
		t.Errorf("Expected title 'First Video', got '%v'", job.Variables["title"])
	}

	if job.Variables["subtitle"] != "Subtitle 1" {
		t.Errorf("Expected subtitle 'Subtitle 1', got '%v'", job.Variables["subtitle"])
	}

	if job.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", job.Priority)
	}

	// Verify second job priority
	if jobs[1].Priority != 2 {
		t.Errorf("Expected priority 2 for job2, got %d", jobs[1].Priority)
	}
}

func TestJSONDataSource_Load(t *testing.T) {
	tempDir := t.TempDir()

	// Create test JSON file
	jsonContent := `{
  "template_name": "youtube/educational",
  "jobs": [
    {
      "id": "video1",
      "variables": {
        "title": "First Video",
        "subtitle": "Subtitle 1"
      },
      "priority": 1
    },
    {
      "id": "video2",
      "template_name": "tiktok/fast-facts",
      "variables": {
        "title": "Second Video",
        "subtitle": "Subtitle 2"
      },
      "priority": 2
    }
  ]
}`

	jsonPath := filepath.Join(tempDir, "test.json")
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to write JSON file: %v", err)
	}

	// Create data source
	ds := NewJSONDataSource(jsonPath, "default-template", tempDir)

	// Load jobs
	jobs, err := ds.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("Expected 2 jobs, got %d", len(jobs))
	}

	// Verify first job (uses file-level template)
	job1 := jobs[0]
	if job1.ID != "video1" {
		t.Errorf("Expected ID 'video1', got '%s'", job1.ID)
	}

	if job1.TemplateName != "youtube/educational" {
		t.Errorf("Expected template 'youtube/educational', got '%s'", job1.TemplateName)
	}

	if job1.Variables["title"] != "First Video" {
		t.Errorf("Expected title 'First Video', got '%v'", job1.Variables["title"])
	}

	// Verify second job (overrides with job-level template)
	job2 := jobs[1]
	if job2.TemplateName != "tiktok/fast-facts" {
		t.Errorf("Expected template 'tiktok/fast-facts', got '%s'", job2.TemplateName)
	}
}

func TestMemoryDataSource(t *testing.T) {
	// Create jobs
	jobs := []*BatchJob{
		{
			ID:           "job1",
			TemplateName: "template1",
			Variables:    map[string]interface{}{"title": "Test 1"},
		},
		{
			ID:           "job2",
			TemplateName: "template2",
			Variables:    map[string]interface{}{"title": "Test 2"},
		},
	}

	// Create data source
	ds := NewMemoryDataSource("test", jobs)

	// Load jobs
	loaded, err := ds.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("Expected 2 jobs, got %d", len(loaded))
	}

	// Verify jobs are same
	for i := range jobs {
		if loaded[i].ID != jobs[i].ID {
			t.Errorf("Job %d: expected ID '%s', got '%s'", i, jobs[i].ID, loaded[i].ID)
		}
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"123", int64(123)},
		{"123.45", float64(123.45)},
		{"true", true},
		{"false", false},
		{"hello", "hello"},
		{"", ""},
		{"  123  ", int64(123)},
		{"  hello  ", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseValue(tt.input)

			switch expected := tt.expected.(type) {
			case int64:
				if r, ok := result.(int64); !ok || r != expected {
					t.Errorf("Expected int64 %v, got %v (%T)", expected, result, result)
				}
			case float64:
				if r, ok := result.(float64); !ok || r != expected {
					t.Errorf("Expected float64 %v, got %v (%T)", expected, result, result)
				}
			case bool:
				if r, ok := result.(bool); !ok || r != expected {
					t.Errorf("Expected bool %v, got %v (%T)", expected, result, result)
				}
			case string:
				if r, ok := result.(string); !ok || r != expected {
					t.Errorf("Expected string '%s', got '%v' (%T)", expected, result, result)
				}
			}
		})
	}
}

func TestGenerateJSONTemplate(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "template.json")

	err := GenerateJSONTemplate(outputPath, 3)
	if err != nil {
		t.Fatalf("GenerateJSONTemplate failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("Template file not created: %v", err)
	}

	// Parse and verify
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read template: %v", err)
	}

	var batch JSONBatchData
	if err := json.Unmarshal(data, &batch); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	if len(batch.Jobs) != 3 {
		t.Errorf("Expected 3 jobs, got %d", len(batch.Jobs))
	}

	if batch.TemplateName != "youtube/educational" {
		t.Errorf("Expected template 'youtube/educational', got '%s'", batch.TemplateName)
	}
}

func TestGenerateCSVTemplate(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "template.csv")

	err := GenerateCSVTemplate(outputPath, 3)
	if err != nil {
		t.Fatalf("GenerateCSVTemplate failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("Template file not created: %v", err)
	}

	// Parse and verify
	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	// Should have header + 3 data rows
	if len(records) != 4 {
		t.Errorf("Expected 4 rows (header + 3 data), got %d", len(records))
	}

	// Verify header
	if records[0][0] != "id" {
		t.Errorf("Expected first column 'id', got '%s'", records[0][0])
	}
}

// Benchmark tests

func BenchmarkBatchProgress_GetStats(b *testing.B) {
	progress := NewBatchProgress()
	progress.TotalJobs = 1000
	progress.CompletedJobs = 500
	progress.FailedJobs = 50
	progress.ActiveJobs = 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = progress.GetStats()
	}
}

func BenchmarkGenerateBatchReport(b *testing.B) {
	// Create sample results
	results := make([]*BatchResult, 100)
	for i := 0; i < 100; i++ {
		results[i] = &BatchResult{
			Job:      &BatchJob{ID: fmt.Sprintf("job%d", i)},
			Success:  i%3 != 0, // 33% failure rate
			Duration: time.Second * time.Duration(i%10+1),
			Error:    "sample error",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateBatchReport(results)
	}
}

func BenchmarkParseValue(b *testing.B) {
	testStrings := []string{"123", "123.45", "true", "hello", ""}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range testStrings {
			_ = parseValue(s)
		}
	}
}
