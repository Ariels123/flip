# FLIP2 Development Patterns & Skills

**Last Updated:** 2025-12-31
**Session:** Option A + B Implementation

---

## 1. Gemini Flash Delegation Pattern

### What Works ✅
- **Analysis & Research**: Excellent at reading code, understanding architecture
- **Recommendations**: Good tech stack choices (Alpine.js, TailwindCSS)
- **Design Exploration**: Can evaluate multiple approaches
- **Documentation Review**: Finds relevant patterns in existing code

### What Doesn't Work ❌
- **File Creation**: Often analyzes but doesn't create files
- **Code Implementation**: Weak at writing complete, working code
- **Multi-step Tasks**: Gets stuck on single steps, doesn't complete sequences
- **Error Recovery**: Doesn't fix issues when tool calls fail

### Optimal Usage Pattern
```
1. Delegate: Research, design, analysis tasks to Gemini
2. Monitor: Check output after 30-60 seconds
3. Decide: If Gemini only analyzed, implement directly
4. Verify: Review Gemini's recommendations, optimize
```

**Example from Session:**
- ✅ Gemini researched PocketBase UI patterns → Good recommendations
- ❌ Gemini didn't create cost tracker files → I implemented directly
- ✅ Gemini suggested Alpine.js + no build step → Excellent choice
- ❌ Gemini didn't write dashboard design doc → I wrote comprehensive spec

---

## 2. Code Review Integration (Antigravity)

### Workflow
```
1. Complete implementation
2. Spawn Antigravity for architecture review
3. Receive structured feedback with severity levels
4. Fix critical/high issues before testing
5. Commit with detailed changelog
```

### Review Categories from AG
- **Best Practices**: Code patterns, error handling
- **Performance**: Memory allocation, algorithmic complexity
- **Security**: Input validation, race conditions
- **Architecture**: Scalability, design improvements

### Fixes Applied This Session

**Critical:**
- Config validation to prevent A→B→A infinite loops
- Fixed infinite loop in communication monitor (corrected != fromAgent check)

**Performance:**
- Levenshtein: O(N*M) matrix → O(min(N,M)) two-row algorithm
- Reduced GC pressure from repeated allocations

**Code Quality:**
- Removed hardcoded defaults that override user config
- Removed 200+ lines of deprecated polling code
- Net -110 lines (cleaner, more maintainable)

---

## 3. PocketBase Patterns

### Collection Creation Pattern
```go
// pb_migrations/N_descriptive_name.go
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("collection_name")

		// Add fields
		collection.Fields.Add(&core.TextField{Name: "field_name", Required: true})
		collection.Fields.Add(&core.NumberField{Name: "count", Min: types.Pointer(0.0)})
		collection.Fields.Add(&core.DateField{Name: "timestamp", Required: true})

		// Add indexes
		collection.AddIndex("idx_name_field", false, "field_name", "")

		// Set rules (empty string = public access)
		collection.ListRule = types.Pointer("")
		collection.ViewRule = types.Pointer("")
		collection.CreateRule = types.Pointer("")

		return app.Save(collection)
	}, func(app core.App) error {
		// Rollback
		collection, err := app.FindCollectionByNameOrId("collection_name")
		if err != nil {
			return nil // Already deleted
		}
		return app.Delete(collection)
	})
}
```

### Store Pattern
```go
// internal/package/pbstore.go
type PBStore struct {
	app        *pocketbase.PocketBase
	collection string
}

func NewPBStore(app *pocketbase.PocketBase) *PBStore {
	return &PBStore{
		app:        app,
		collection: "collection_name",
	}
}

func (s *PBStore) Query(ctx context.Context, filter string) ([]Record, error) {
	collection, err := s.app.FindCollectionByNameOrId(s.collection)
	if err != nil {
		return nil, fmt.Errorf("collection not found: %w", err)
	}

	records, err := s.app.FindRecordsByFilter(
		collection.Id,
		filter,
		"-timestamp", // Sort descending
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Convert to domain objects
	results := make([]Record, 0, len(records))
	for _, r := range records {
		results = append(results, recordToDomain(r))
	}
	return results, nil
}
```

### Important Gotchas

**System Fields:**
- ❌ Don't use `@created` or `@updated` in filters/sorts
- ✅ Use plain `created` and `updated` for user-defined fields
- ✅ Use `-id` for time-based sorting (IDs are time-sortable)

**Filter Syntax:**
```go
// Correct
filter := fmt.Sprintf("agent_id = '%s' && timestamp >= '%s'",
	agentID, start.Format(time.RFC3339))

// Wrong - @ prefix not needed for filters (only in schemas)
filter := fmt.Sprintf("@created >= '%s'", start.Format(time.RFC3339))
```

---

## 4. FLIP2 Architecture Patterns

### Daemon Integration Pattern

**1. Create package in internal/**
```
internal/
├── costtracker/
│   ├── costtracker.go  # Core business logic
│   └── pbstore.go      # PocketBase adapter
```

**2. Add to daemon initialization:**
```go
// internal/daemon/daemon.go
type Daemon struct {
	// ... existing fields
	costTracker *costtracker.Tracker
}

func New(configPath string) (*Daemon, error) {
	// ... existing init

	// Initialize cost tracker
	costStore := costtracker.NewPBStore(d.pb)
	d.costTracker = costtracker.New(costStore, d.logger)

	return d, nil
}
```

**3. Hook into events:**
```go
// After LLM execution in handlers
d.costTracker.RecordCost(ctx,
	agentID,
	response.Model,
	taskID,
	response.InputTokens,
	response.OutputTokens,
	response.CostUSD,
)
```

### Component Patterns

**Archiver Pattern:**
- Runs on timer (6 hour intervals)
- Filters by age and agent type
- Moves to archive collection, then file storage
- Critical: Don't fail tasks on archiving errors

**CommMonitor Pattern:**
- Event-driven via PocketBase hooks
- OnRecordAfterCreateSuccess + OnRecordAfterUpdateSuccess
- No polling loop (deprecated)
- Fail-safe: Log errors, don't block signal processing

**CostTracker Pattern:**
- Record on every LLM call
- Store raw data (tokens, costs)
- Aggregate on-demand via queries
- Support daily/monthly/agent/model breakdowns

---

## 5. Dashboard Design Patterns

### No-Build-Step Architecture

**Why:**
- Faster iteration (edit → refresh)
- No Node.js dependency
- Simpler deployment
- Works on any static file server

**How:**
```html
<!-- All dependencies via CDN -->
<script src="https://cdn.tailwindcss.com"></script>
<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.x.x"></script>
<script src="https://cdn.jsdelivr.net/npm/pocketbase@0.21.x/dist/pocketbase.umd.js"></script>
```

### Alpine.js Component Pattern

```javascript
document.addEventListener('alpine:init', () => {
	Alpine.data('componentName', () => ({
		// State
		data: [],
		loading: false,

		// Lifecycle
		init() {
			this.loadData()
			this.subscribe()
		},

		// Methods
		async loadData() {
			this.loading = true
			this.data = await fetch('/api/endpoint')
			this.loading = false
		},

		// Real-time
		subscribe() {
			pb.collection('name').subscribe('*', (e) => {
				this.handleUpdate(e)
			})
		}
	}))
})
```

### TailwindCSS Responsive Pattern

```html
<!-- Mobile: 1 col, Tablet: 2 col, Desktop: 4 col -->
<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
	<div class="bg-slate-800 p-6 rounded-lg">
		<!-- Card content -->
	</div>
</div>
```

### Chart.js Integration

```javascript
new Chart(ctx, {
	type: 'line',
	data: {
		labels: timestamps,
		datasets: [{
			label: 'Signals/min',
			data: values,
			borderColor: '#8b5cf6',
			tension: 0.4
		}]
	},
	options: {
		responsive: true,
		maintainAspectRatio: false,
		plugins: {
			legend: { display: false }
		}
	}
})
```

---

## 6. Error Handling Patterns

### Fail-Safe Pattern (Archiver, CommMonitor)
```go
// Log errors but don't fail the main task
if err := optional.Operation(); err != nil {
	logger.Error("Optional operation failed", "error", err)
	// Continue execution
}
```

### Fail-Fast Pattern (Config Validation)
```go
// Fail on startup if config is invalid
if err := validateConfig(); err != nil {
	return nil, fmt.Errorf("invalid config: %w", err)
}
```

### Error Metrics Pattern
```go
type Component struct {
	mu            sync.RWMutex
	errorCount    int
	lastError     error
	lastErrorTime time.Time
}

func (c *Component) recordError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorCount++
	c.lastError = err
	c.lastErrorTime = time.Now()
}

func (c *Component) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var lastErrorStr string
	if c.lastError != nil {
		lastErrorStr = c.lastError.Error()
	}

	return map[string]interface{}{
		"error_count": c.errorCount,
		"last_error": lastErrorStr,
		"last_error_time": c.lastErrorTime,
	}
}
```

---

## 7. Git Commit Patterns

### Commit Message Structure
```
Title: Brief summary (50 chars)

Detailed description with sections:

## Section 1: Feature/Fix Name

Description...

## Section 2: Related Changes

Description...

## Files Modified:
- file1: changes
- file2: changes

## Testing:
✅ Test description

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
```

### When to Commit
- After completing logical unit of work
- All tests passing (if applicable)
- Code review feedback addressed
- Before switching to different feature

---

## 8. Performance Patterns

### Memory Optimization
```go
// Before: O(N*M) memory
matrix := make([][]int, len(s1)+1)
for i := range matrix {
	matrix[i] = make([]int, len(s2)+1)
}

// After: O(min(N,M)) memory
v0 := make([]int, len(s2)+1)
v1 := make([]int, len(s2)+1)
// Reuse rows in loop
```

### Map vs Array Lookups
```go
// Before: O(N) lookup
validAgents := []string{"agent1", "agent2", "agent3"}
for _, valid := range validAgents {
	if agent == valid {
		return true
	}
}

// After: O(1) lookup
validAgents := map[string]bool{
	"agent1": true,
	"agent2": true,
	"agent3": true,
}
return validAgents[strings.ToLower(agent)]
```

---

## 9. Configuration Patterns

### Config Migration Pattern
```go
// 1. Add struct field
type Config struct {
	NewFeature NewFeatureConfig `yaml:"new_feature"`
}

// 2. Add defaults
if config.NewFeature.Enabled && config.NewFeature.Field == "" {
	config.NewFeature.Field = "default_value"
}

// 3. Validate
if err := validateNewFeatureConfig(&config.NewFeature); err != nil {
	return nil, fmt.Errorf("invalid config: %w", err)
}
```

### Validation Pattern
```go
func validateConfig(cfg *Config) error {
	// Build lookup maps
	validItems := make(map[string]bool)
	for _, item := range cfg.ValidItems {
		validItems[strings.ToLower(item)] = true
	}

	// Validate references
	for key, target := range cfg.Mappings {
		if !validItems[strings.ToLower(target)] {
			return fmt.Errorf(
				"mapping '%s' -> '%s' invalid: target not in valid list",
				key, target,
			)
		}
	}

	return nil
}
```

---

## 10. Testing Patterns

### Daemon Testing Pattern
```bash
# Build
go build -o flip2d cmd/flip2d/main.go

# Test startup
./flip2d --config config/config.yaml --foreground 2>&1 | grep -i "error\|success" | head -20 &
sleep 5
pkill -f flip2d

# Check logs for specific component
./flip2d --config config/config.yaml --foreground 2>&1 | grep "component_name" | head -20 &
```

### Database Schema Testing
```bash
# Check collection exists
sqlite3 pb_data/data.db "SELECT sql FROM sqlite_master WHERE type='table' AND name='costs';"

# Count records
sqlite3 pb_data/data.db "SELECT COUNT(*) FROM costs;"

# Sample data
sqlite3 pb_data/data.db "SELECT * FROM costs LIMIT 5;"
```

---

## Quick Reference

### File Locations
```
flip2/
├── cmd/flip2d/main.go           # Daemon entry point
├── config/config.yaml           # Runtime configuration
├── internal/
│   ├── daemon/daemon.go         # Main daemon initialization
│   ├── config/config.go         # Config parsing
│   ├── api/handlers.go          # HTTP API handlers
│   ├── archiver/                # Message archiving
│   ├── commmonitor/             # Typo correction
│   ├── costtracker/             # Cost tracking (NEW)
│   └── executor/                # Task execution
├── pb_migrations/               # Database migrations
├── pb_public/                   # Static files (dashboard)
└── docs/                        # Design documents
```

### Common Commands
```bash
# Build
go build -o flip2d cmd/flip2d/main.go

# Run foreground
./flip2d --config config/config.yaml --foreground

# Test compilation without running
go build -o /tmp/test cmd/flip2d/main.go

# Find where function is used
find internal -name "*.go" | xargs grep -l "FunctionName"

# Check collection schema
grep -A 20 "collection_name :=" pb_migrations/*.go
```

---

## Session Summary: Option A + B

**Time:** ~4 hours
**Tasks Completed:** 10 (7 Option A, 2 Option B, 1 code review)
**Net Code Change:** -110 lines (cleaner, better)
**Gemini Cost:** $0.000315
**Pattern Learned:** Gemini for research, Claude for implementation

**Key Achievements:**
1. Fixed critical infinite loop bug
2. Optimized Levenshtein algorithm
3. Migrated config to YAML
4. Removed deprecated code
5. Created cost tracking system
6. Designed monitoring dashboard

**Files Created:** 5 new files, 5 modified
**Quality:** Code review passed, daemon tested successfully
