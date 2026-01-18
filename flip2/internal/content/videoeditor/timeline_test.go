package videoeditor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"flip2/internal/logger"
)

func TestTimelineExecutor_ValidateTimeline(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	// Create a test video file for validation
	testVideoPath := filepath.Join(tempDir, "test.mp4")
	if err := os.WriteFile(testVideoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	tests := []struct {
		name          string
		template      *Template
		expectError   bool
		errorContains string
	}{
		{
			name: "valid timeline",
			template: &Template{
				Name:    "valid",
				Version: "1.0",
				Canvas: CanvasConfig{
					Width:  1920,
					Height: 1080,
					FPS:    30,
				},
				Timeline: []TimelineItem{
					{
						Type:      "video",
						Path:      testVideoPath,
						StartTime: 0,
						Duration:  5000,
					},
					{
						Type:      "text",
						Content:   "Hello",
						StartTime: 1000,
						Duration:  3000,
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty timeline",
			template: &Template{
				Name:    "empty",
				Version: "1.0",
				Canvas: CanvasConfig{
					Width:  1920,
					Height: 1080,
					FPS:    30,
				},
				Timeline: []TimelineItem{},
			},
			expectError:   true,
			errorContains: "timeline is empty",
		},
		{
			name: "negative start time",
			template: &Template{
				Name:    "negative",
				Version: "1.0",
				Canvas: CanvasConfig{
					Width:  1920,
					Height: 1080,
					FPS:    30,
				},
				Timeline: []TimelineItem{
					{
						Type:      "text",
						Content:   "Hello",
						StartTime: -1000,
						Duration:  3000,
					},
				},
			},
			expectError:   true,
			errorContains: "negative start time",
		},
		{
			name: "zero duration",
			template: &Template{
				Name:    "zero-duration",
				Version: "1.0",
				Canvas: CanvasConfig{
					Width:  1920,
					Height: 1080,
					FPS:    30,
				},
				Timeline: []TimelineItem{
					{
						Type:      "text",
						Content:   "Hello",
						StartTime: 0,
						Duration:  0,
					},
				},
			},
			expectError:   true,
			errorContains: "non-positive duration",
		},
		{
			name: "missing video file",
			template: &Template{
				Name:    "missing-file",
				Version: "1.0",
				Canvas: CanvasConfig{
					Width:  1920,
					Height: 1080,
					FPS:    30,
				},
				Timeline: []TimelineItem{
					{
						Type:      "video",
						Path:      "/nonexistent/video.mp4",
						StartTime: 0,
						Duration:  5000,
					},
				},
			},
			expectError:   true,
			errorContains: "file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := NewVideoOperations("ffmpeg", "ffprobe", tempDir)
			te := NewTimelineExecutor(ops, tempDir, log)

			err := te.validateTimeline(tt.template)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestTimelineExecutor_BuildExecutionPlan(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	template := &Template{
		Name:    "test",
		Version: "1.0",
		Canvas: CanvasConfig{
			Width:  1920,
			Height: 1080,
			FPS:    30,
		},
		Timeline: []TimelineItem{
			{
				Type:      "video",
				Path:      "/path/to/video1.mp4",
				StartTime: 0,
				Duration:  5000,
			},
			{
				Type:      "text",
				Content:   "Hello",
				StartTime: 1000,
				Duration:  3000,
			},
			{
				Type:      "video",
				Path:      "/path/to/video2.mp4",
				StartTime: 5000,
				Duration:  3000,
			},
		},
	}

	ops := NewVideoOperations("ffmpeg", "ffprobe", tempDir)
	te := NewTimelineExecutor(ops, tempDir, log)

	plan, err := te.buildExecutionPlan(context.Background(), template)
	if err != nil {
		t.Fatalf("buildExecutionPlan failed: %v", err)
	}

	// Verify plan structure
	if len(plan.Stages) != len(template.Timeline) {
		t.Errorf("Expected %d stages, got %d", len(template.Timeline), len(plan.Stages))
	}

	// Verify total duration calculation
	expectedDuration := 8000.0 // max(5000, 1000+3000, 5000+3000) = 8000
	if plan.TotalDuration != expectedDuration {
		t.Errorf("Expected total duration %.2f, got %.2f", expectedDuration, plan.TotalDuration)
	}

	// Verify stage types
	if plan.Stages[0].Type != "video" {
		t.Errorf("Expected stage 0 type 'video', got '%s'", plan.Stages[0].Type)
	}

	if plan.Stages[1].Type != "text" {
		t.Errorf("Expected stage 1 type 'text', got '%s'", plan.Stages[1].Type)
	}

	if plan.Stages[2].Type != "video" {
		t.Errorf("Expected stage 2 type 'video', got '%s'", plan.Stages[2].Type)
	}
}

func TestTimelineExecutor_BuildExecutionPlan_TotalDuration(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	tests := []struct {
		name             string
		timeline         []TimelineItem
		expectedDuration float64
	}{
		{
			name: "sequential items",
			timeline: []TimelineItem{
				{Type: "video", Path: "a.mp4", StartTime: 0, Duration: 5000},
				{Type: "video", Path: "b.mp4", StartTime: 5000, Duration: 5000},
			},
			expectedDuration: 10000,
		},
		{
			name: "overlapping items",
			timeline: []TimelineItem{
				{Type: "video", Path: "a.mp4", StartTime: 0, Duration: 8000},
				{Type: "text", Content: "Hello", StartTime: 2000, Duration: 3000},
			},
			expectedDuration: 8000,
		},
		{
			name: "gap between items",
			timeline: []TimelineItem{
				{Type: "video", Path: "a.mp4", StartTime: 0, Duration: 3000},
				{Type: "video", Path: "b.mp4", StartTime: 5000, Duration: 3000},
			},
			expectedDuration: 8000,
		},
		{
			name: "single item",
			timeline: []TimelineItem{
				{Type: "video", Path: "a.mp4", StartTime: 0, Duration: 5000},
			},
			expectedDuration: 5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &Template{
				Name:     "test",
				Version:  "1.0",
				Canvas:   CanvasConfig{Width: 1920, Height: 1080, FPS: 30},
				Timeline: tt.timeline,
			}

			ops := NewVideoOperations("ffmpeg", "ffprobe", tempDir)
			te := NewTimelineExecutor(ops, tempDir, log)

			plan, err := te.buildExecutionPlan(context.Background(), template)
			if err != nil {
				t.Fatalf("buildExecutionPlan failed: %v", err)
			}

			if plan.TotalDuration != tt.expectedDuration {
				t.Errorf("Expected total duration %.2f, got %.2f", tt.expectedDuration, plan.TotalDuration)
			}
		})
	}
}

func TestTimelineExecutor_CreateBlankCanvas(t *testing.T) {
	// Skip if ffmpeg not available
	if _, err := os.Stat("/usr/bin/ffmpeg"); err != nil && os.Getenv("FFMPEG_PATH") == "" {
		t.Skip("ffmpeg not available, skipping test")
	}

	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	canvas := CanvasConfig{
		Width:  1280,
		Height: 720,
		FPS:    30,
	}

	ops := NewVideoOperations("ffmpeg", "ffprobe", tempDir)
	te := NewTimelineExecutor(ops, tempDir, log)

	outputPath, err := te.createBlankCanvas(context.Background(), canvas, 2.0)
	if err != nil {
		t.Fatalf("createBlankCanvas failed: %v", err)
	}

	// Verify output file exists
	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("Output file not created: %v", err)
	}

	// Clean up
	os.Remove(outputPath)
}

func TestTimelineExecutor_VariableSubstitution(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	// Create test video file
	testVideoPath := filepath.Join(tempDir, "test.mp4")
	if err := os.WriteFile(testVideoPath, []byte("fake video"), 0644); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	template := &Template{
		Name:    "test",
		Version: "1.0",
		Canvas: CanvasConfig{
			Width:  1920,
			Height: 1080,
			FPS:    30,
		},
		Timeline: []TimelineItem{
			{
				Type:      "video",
				Path:      "{{video_path}}",
				StartTime: 0,
				Duration:  5000,
			},
			{
				Type:      "text",
				Content:   "Welcome to {{title}}",
				StartTime: 1000,
				Duration:  3000,
				Style: &TextStyle{
					FontSize:  64,
					FontColor: "{{brand_color}}",
					Position:  "center",
				},
			},
		},
		Variables: map[string]interface{}{
			"video_path":  testVideoPath,
			"title":       "My Video",
			"brand_color": "#FF0000",
		},
	}

	ops := NewVideoOperations("ffmpeg", "ffprobe", tempDir)
	te := NewTimelineExecutor(ops, tempDir, log)

	// Create variable substitutor and substitute template
	te.substitutor = NewVariableSubstitutor(template.Variables)
	processedTemplate, err := te.substitutor.SubstituteTemplate(template)
	if err != nil {
		t.Fatalf("Variable substitution failed: %v", err)
	}

	// Verify substitution
	if processedTemplate.Timeline[0].Path != testVideoPath {
		t.Errorf("Expected path '%s', got '%s'", testVideoPath, processedTemplate.Timeline[0].Path)
	}

	if processedTemplate.Timeline[1].Content != "Welcome to My Video" {
		t.Errorf("Expected content 'Welcome to My Video', got '%s'", processedTemplate.Timeline[1].Content)
	}

	if processedTemplate.Timeline[1].Style.FontColor != "#FF0000" {
		t.Errorf("Expected font color '#FF0000', got '%s'", processedTemplate.Timeline[1].Style.FontColor)
	}

	// Verify original template is unchanged
	if template.Timeline[0].Path != "{{video_path}}" {
		t.Errorf("Original template was modified")
	}
}

func TestTimelineExecutor_ItemGrouping(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	template := &Template{
		Name:    "test",
		Version: "1.0",
		Canvas: CanvasConfig{
			Width:  1920,
			Height: 1080,
			FPS:    30,
		},
		Timeline: []TimelineItem{
			{Type: "video", Path: "a.mp4", StartTime: 0, Duration: 5000},
			{Type: "text", Content: "Hello", StartTime: 1000, Duration: 3000},
			{Type: "image", Path: "img.png", StartTime: 5000, Duration: 3000},
			{Type: "audio", Path: "audio.mp3", StartTime: 0, Duration: 8000},
			{Type: "text", Content: "World", StartTime: 6000, Duration: 2000},
		},
	}

	ops := NewVideoOperations("ffmpeg", "ffprobe", tempDir)
	te := NewTimelineExecutor(ops, tempDir, log)

	// Build execution plan
	plan, err := te.buildExecutionPlan(context.Background(), template)
	if err != nil {
		t.Fatalf("buildExecutionPlan failed: %v", err)
	}

	// Count items by type
	videoCount := 0
	textCount := 0
	audioCount := 0

	for _, stage := range plan.Stages {
		switch stage.Type {
		case "video", "image":
			videoCount++
		case "text":
			textCount++
		case "audio":
			audioCount++
		}
	}

	if videoCount != 2 {
		t.Errorf("Expected 2 video/image items, got %d", videoCount)
	}

	if textCount != 2 {
		t.Errorf("Expected 2 text items, got %d", textCount)
	}

	if audioCount != 1 {
		t.Errorf("Expected 1 audio item, got %d", audioCount)
	}
}

// Benchmark tests

func BenchmarkTimelineExecutor_BuildExecutionPlan(b *testing.B) {
	tempDir := b.TempDir()
	log := (*logger.Logger)(nil)

	template := &Template{
		Name:    "benchmark",
		Version: "1.0",
		Canvas: CanvasConfig{
			Width:  1920,
			Height: 1080,
			FPS:    30,
		},
		Timeline: make([]TimelineItem, 50),
	}

	// Create 50 timeline items
	for i := 0; i < 50; i++ {
		template.Timeline[i] = TimelineItem{
			Type:      "text",
			Content:   "Item " + string(rune(i)),
			StartTime: float64(i * 1000),
			Duration:  1000,
		}
	}

	ops := NewVideoOperations("ffmpeg", "ffprobe", tempDir)
	te := NewTimelineExecutor(ops, tempDir, log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := te.buildExecutionPlan(context.Background(), template)
		if err != nil {
			b.Fatalf("buildExecutionPlan failed: %v", err)
		}
	}
}

func BenchmarkTimelineExecutor_ValidateTimeline(b *testing.B) {
	tempDir := b.TempDir()
	log := (*logger.Logger)(nil)

	// Create test file
	testFile := filepath.Join(tempDir, "test.mp4")
	os.WriteFile(testFile, []byte("fake"), 0644)

	template := &Template{
		Name:    "benchmark",
		Version: "1.0",
		Canvas: CanvasConfig{
			Width:  1920,
			Height: 1080,
			FPS:    30,
		},
		Timeline: []TimelineItem{
			{Type: "video", Path: testFile, StartTime: 0, Duration: 5000},
			{Type: "text", Content: "Hello", StartTime: 1000, Duration: 3000},
		},
	}

	ops := NewVideoOperations("ffmpeg", "ffprobe", tempDir)
	te := NewTimelineExecutor(ops, tempDir, log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := te.validateTimeline(template)
		if err != nil {
			b.Fatalf("validateTimeline failed: %v", err)
		}
	}
}
