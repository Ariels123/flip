// Package videoeditor - Data source adapters for batch processing (Phase 4)
// Enables data-driven video generation from CSV, JSON, and APIs
package videoeditor

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// DataSource defines the interface for batch data sources
type DataSource interface {
	// Load loads data and returns batch jobs
	Load() ([]*BatchJob, error)

	// Name returns the data source name
	Name() string
}

// --- CSV Data Source ---

// CSVDataSource reads batch jobs from a CSV file
type CSVDataSource struct {
	filePath     string
	templateName string
	outputDir    string

	// Column mappings
	idColumn       int
	variableColumns map[string]int
	priorityColumn  int
}

// NewCSVDataSource creates a new CSV data source
func NewCSVDataSource(filePath, templateName, outputDir string) *CSVDataSource {
	return &CSVDataSource{
		filePath:        filePath,
		templateName:    templateName,
		outputDir:       outputDir,
		variableColumns: make(map[string]int),
	}
}

// SetColumnMapping sets column mappings for CSV
func (ds *CSVDataSource) SetColumnMapping(idCol int, variableCols map[string]int, priorityCol int) {
	ds.idColumn = idCol
	ds.variableColumns = variableCols
	ds.priorityColumn = priorityCol
}

// Load reads the CSV file and creates batch jobs
func (ds *CSVDataSource) Load() ([]*BatchJob, error) {
	file, err := os.Open(ds.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Auto-detect column mappings if not set
	if len(ds.variableColumns) == 0 {
		ds.autoDetectColumns(header)
	}

	jobs := make([]*BatchJob, 0)

	// Read data rows
	rowNum := 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row %d: %w", rowNum, err)
		}

		// Extract job data
		jobID := fmt.Sprintf("job_%d", rowNum)
		if ds.idColumn >= 0 && ds.idColumn < len(record) {
			jobID = record[ds.idColumn]
		}

		variables := make(map[string]interface{})
		for varName, colIdx := range ds.variableColumns {
			if colIdx >= 0 && colIdx < len(record) {
				variables[varName] = parseValue(record[colIdx])
			}
		}

		priority := 0
		if ds.priorityColumn >= 0 && ds.priorityColumn < len(record) {
			priority, _ = strconv.Atoi(record[ds.priorityColumn])
		}

		outputPath := fmt.Sprintf("%s/%s.mp4", ds.outputDir, jobID)

		job := &BatchJob{
			ID:           jobID,
			TemplateName: ds.templateName,
			Variables:    variables,
			OutputPath:   outputPath,
			Priority:     priority,
			Metadata: map[string]interface{}{
				"source":     "csv",
				"row_number": rowNum,
			},
		}

		jobs = append(jobs, job)
		rowNum++
	}

	return jobs, nil
}

// autoDetectColumns attempts to detect column mappings from header
func (ds *CSVDataSource) autoDetectColumns(header []string) {
	for i, colName := range header {
		colName = strings.ToLower(strings.TrimSpace(colName))

		switch colName {
		case "id", "job_id":
			ds.idColumn = i
		case "priority":
			ds.priorityColumn = i
		default:
			// Treat as variable column
			ds.variableColumns[colName] = i
		}
	}
}

// Name returns the data source name
func (ds *CSVDataSource) Name() string {
	return fmt.Sprintf("CSV: %s", ds.filePath)
}

// --- JSON Data Source ---

// JSONDataSource reads batch jobs from a JSON file
type JSONDataSource struct {
	filePath     string
	templateName string
	outputDir    string
}

// NewJSONDataSource creates a new JSON data source
func NewJSONDataSource(filePath, templateName, outputDir string) *JSONDataSource {
	return &JSONDataSource{
		filePath:     filePath,
		templateName: templateName,
		outputDir:    outputDir,
	}
}

// JSONBatchData represents the JSON file structure
type JSONBatchData struct {
	// TemplateName overrides the data source template
	TemplateName string `json:"template_name,omitempty"`

	// Jobs is the list of job definitions
	Jobs []JSONJobDefinition `json:"jobs"`
}

// JSONJobDefinition defines a job in JSON
type JSONJobDefinition struct {
	ID           string                 `json:"id"`
	TemplateName string                 `json:"template_name,omitempty"`
	Variables    map[string]interface{} `json:"variables"`
	OutputPath   string                 `json:"output_path,omitempty"`
	Priority     int                    `json:"priority,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Load reads the JSON file and creates batch jobs
func (ds *JSONDataSource) Load() ([]*BatchJob, error) {
	file, err := os.Open(ds.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSON file: %w", err)
	}
	defer file.Close()

	var data JSONBatchData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Use file-level template if specified
	templateName := ds.templateName
	if data.TemplateName != "" {
		templateName = data.TemplateName
	}

	jobs := make([]*BatchJob, 0, len(data.Jobs))

	for i, jobDef := range data.Jobs {
		// Use job-level template if specified
		jobTemplate := templateName
		if jobDef.TemplateName != "" {
			jobTemplate = jobDef.TemplateName
		}

		// Generate output path if not specified
		outputPath := jobDef.OutputPath
		if outputPath == "" {
			jobID := jobDef.ID
			if jobID == "" {
				jobID = fmt.Sprintf("job_%d", i)
			}
			outputPath = fmt.Sprintf("%s/%s.mp4", ds.outputDir, jobID)
		}

		// Generate job ID if not specified
		jobID := jobDef.ID
		if jobID == "" {
			jobID = fmt.Sprintf("job_%d", i)
		}

		job := &BatchJob{
			ID:           jobID,
			TemplateName: jobTemplate,
			Variables:    jobDef.Variables,
			OutputPath:   outputPath,
			Priority:     jobDef.Priority,
			Metadata:     jobDef.Metadata,
		}

		if job.Metadata == nil {
			job.Metadata = make(map[string]interface{})
		}
		job.Metadata["source"] = "json"
		job.Metadata["index"] = i

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// Name returns the data source name
func (ds *JSONDataSource) Name() string {
	return fmt.Sprintf("JSON: %s", ds.filePath)
}

// --- In-Memory Data Source ---

// MemoryDataSource holds jobs in memory
type MemoryDataSource struct {
	jobs []*BatchJob
	name string
}

// NewMemoryDataSource creates a new in-memory data source
func NewMemoryDataSource(name string, jobs []*BatchJob) *MemoryDataSource {
	return &MemoryDataSource{
		jobs: jobs,
		name: name,
	}
}

// Load returns the in-memory jobs
func (ds *MemoryDataSource) Load() ([]*BatchJob, error) {
	return ds.jobs, nil
}

// Name returns the data source name
func (ds *MemoryDataSource) Name() string {
	return fmt.Sprintf("Memory: %s", ds.name)
}

// --- Helper Functions ---

// parseValue attempts to parse a string value to appropriate type
func parseValue(s string) interface{} {
	s = strings.TrimSpace(s)

	// Try integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}

	// Try float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// Try boolean
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}

	// Default to string
	return s
}

// GenerateJSONTemplate generates a sample JSON batch data file
func GenerateJSONTemplate(outputPath string, count int) error {
	data := JSONBatchData{
		TemplateName: "youtube/educational",
		Jobs:         make([]JSONJobDefinition, count),
	}

	for i := 0; i < count; i++ {
		data.Jobs[i] = JSONJobDefinition{
			ID: fmt.Sprintf("video_%d", i+1),
			Variables: map[string]interface{}{
				"title":        fmt.Sprintf("Video Title %d", i+1),
				"subtitle":     fmt.Sprintf("Subtitle %d", i+1),
				"main_content": fmt.Sprintf("/path/to/video_%d.mp4", i+1),
			},
			Priority: 0,
		}
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// GenerateCSVTemplate generates a sample CSV batch data file
func GenerateCSVTemplate(outputPath string, count int) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"id", "title", "subtitle", "main_content", "priority"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data rows
	for i := 0; i < count; i++ {
		row := []string{
			fmt.Sprintf("video_%d", i+1),
			fmt.Sprintf("Video Title %d", i+1),
			fmt.Sprintf("Subtitle %d", i+1),
			fmt.Sprintf("/path/to/video_%d.mp4", i+1),
			"0",
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}
