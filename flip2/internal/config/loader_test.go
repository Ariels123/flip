package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewConfigLoader tests basic initialization
func TestNewConfigLoader(t *testing.T) {
	tmpDir := t.TempDir()

	loader, err := NewConfigLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewConfigLoader failed: %v", err)
	}

	if loader == nil {
		t.Fatal("loader is nil")
	}

	paths := loader.GetLoadPaths()
	if paths.Directory != filepath.Join(tmpDir, "FLIP2.md") {
		t.Errorf("directory path incorrect: got %s, want %s", paths.Directory, filepath.Join(tmpDir, "FLIP2.md"))
	}
}

// TestLoadConfigChain_AllLevelsEmpty tests when no configs exist
func TestLoadConfigChain_AllLevelsEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	loader, err := NewConfigLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewConfigLoader failed: %v", err)
	}

	merged, err := loader.LoadConfigChain()
	if err != nil {
		t.Fatalf("LoadConfigChain failed: %v", err)
	}

	if merged.MergeInfo.GlobalLoaded || merged.MergeInfo.ProjectLoaded || merged.MergeInfo.DirectoryLoaded {
		t.Fatal("expected no configs to be loaded")
	}

	if len(merged.Agents) != 0 || len(merged.Commands) != 0 || len(merged.Routes) != 0 {
		t.Fatal("expected empty merged config")
	}
}

// TestLoadConfigChain_GlobalOnly tests loading only global config
func TestLoadConfigChain_GlobalOnly(t *testing.T) {
	globalDir, globalFile := createTestGlobalConfig(t)
	defer os.RemoveAll(globalDir)

	workDir := t.TempDir()
	loader, err := NewConfigLoader(workDir)
	if err != nil {
		t.Fatalf("NewConfigLoader failed: %v", err)
	}

	// Override the global path to use our test file
	loader.globalPath = globalFile

	merged, err := loader.LoadConfigChain()
	if err != nil {
		t.Fatalf("LoadConfigChain failed: %v", err)
	}

	if !merged.MergeInfo.GlobalLoaded {
		t.Fatal("expected global config to be loaded")
	}
	if merged.MergeInfo.ProjectLoaded || merged.MergeInfo.DirectoryLoaded {
		t.Fatal("expected only global config to be loaded")
	}

	// Check content
	if len(merged.Agents) != 1 || merged.Agents[0].Name != "DefaultAgent" {
		t.Fatal("global agent not properly merged")
	}
	if len(merged.Commands) != 1 || merged.Commands[0].Name != "/default" {
		t.Fatal("global command not properly merged")
	}
}

// TestLoadConfigChain_ProjectOverridesGlobal tests project config overriding global
func TestLoadConfigChain_ProjectOverridesGlobal(t *testing.T) {
	globalDir, globalFile := createTestGlobalConfig(t)
	defer os.RemoveAll(globalDir)

	workDir := t.TempDir()

	// Create a project with go.mod
	createTestProject(t, workDir)

	// Create project config
	projectFile := createTestProjectConfig(t, workDir)

	loader, err := NewConfigLoader(workDir)
	if err != nil {
		t.Fatalf("NewConfigLoader failed: %v", err)
	}

	// Override paths
	loader.globalPath = globalFile
	loader.projectPath = projectFile

	merged, err := loader.LoadConfigChain()
	if err != nil {
		t.Fatalf("LoadConfigChain failed: %v", err)
	}

	if !merged.MergeInfo.GlobalLoaded || !merged.MergeInfo.ProjectLoaded {
		t.Fatal("expected both global and project configs to be loaded")
	}

	// Project command should override global command (same name)
	cmd := merged.GetCommand("/default")
	if cmd == nil {
		t.Fatal("command /default not found")
	}
	if cmd.Handler != "project-handler" {
		t.Errorf("expected project handler, got %s", cmd.Handler)
	}

	// Project agent should override global agent (same name)
	agent := merged.GetAgent("DefaultAgent")
	if agent == nil {
		t.Fatal("agent DefaultAgent not found")
	}
	if agent.Model != "project-model" {
		t.Errorf("expected project model, got %s", agent.Model)
	}
}

// TestLoadConfigChain_DirectoryOverridesAll tests directory config with highest precedence
func TestLoadConfigChain_DirectoryOverridesAll(t *testing.T) {
	globalDir, globalFile := createTestGlobalConfig(t)
	defer os.RemoveAll(globalDir)

	workDir := t.TempDir()

	// Create project
	createTestProject(t, workDir)
	projectFile := createTestProjectConfig(t, workDir)

	// Create directory config
	directoryFile := createTestDirectoryConfig(t, workDir)

	loader, err := NewConfigLoader(workDir)
	if err != nil {
		t.Fatalf("NewConfigLoader failed: %v", err)
	}

	// Override paths
	loader.globalPath = globalFile
	loader.projectPath = projectFile
	loader.directoryPath = directoryFile

	merged, err := loader.LoadConfigChain()
	if err != nil {
		t.Fatalf("LoadConfigChain failed: %v", err)
	}

	if !merged.MergeInfo.GlobalLoaded || !merged.MergeInfo.ProjectLoaded || !merged.MergeInfo.DirectoryLoaded {
		t.Fatal("expected all configs to be loaded")
	}

	// Directory config should override both global and project
	cmd := merged.GetCommand("/default")
	if cmd == nil {
		t.Fatal("command /default not found")
	}
	if cmd.Handler != "directory-handler" {
		t.Errorf("expected directory handler, got %s", cmd.Handler)
	}

	agent := merged.GetAgent("DefaultAgent")
	if agent == nil {
		t.Fatal("agent DefaultAgent not found")
	}
	if agent.Model != "directory-model" {
		t.Errorf("expected directory model, got %s", agent.Model)
	}
}

// TestLoadConfigChain_ContextAccumulates tests that context files accumulate
func TestLoadConfigChain_ContextAccumulates(t *testing.T) {
	globalDir, globalFile := createTestGlobalConfigWithContext(t)
	defer os.RemoveAll(globalDir)

	workDir := t.TempDir()
	createTestProject(t, workDir)
	projectFile := createTestProjectConfigWithContext(t, workDir)

	loader, err := NewConfigLoader(workDir)
	if err != nil {
		t.Fatalf("NewConfigLoader failed: %v", err)
	}

	loader.globalPath = globalFile
	loader.projectPath = projectFile

	merged, err := loader.LoadConfigChain()
	if err != nil {
		t.Fatalf("LoadConfigChain failed: %v", err)
	}

	// Should have context files from both global and project
	if len(merged.Context.AutoLoadFiles) != 2 {
		t.Errorf("expected 2 context files, got %d", len(merged.Context.AutoLoadFiles))
	}

	// Check that both files are present
	paths := make(map[string]bool)
	for _, file := range merged.Context.AutoLoadFiles {
		paths[file.Path] = true
	}

	if !paths["./global/README.md"] {
		t.Fatal("global context file not found")
	}
	if !paths["./project/ARCHITECTURE.md"] {
		t.Fatal("project context file not found")
	}
}

// TestLoadConfigChain_DuplicateContextFiles tests that duplicate context files don't accumulate
func TestLoadConfigChain_DuplicateContextFiles(t *testing.T) {
	globalDir, globalFile := createTestGlobalConfigWithDuplicateContext(t)
	defer os.RemoveAll(globalDir)

	workDir := t.TempDir()
	createTestProject(t, workDir)
	projectFile := createTestProjectConfigWithDuplicateContext(t, workDir)

	loader, err := NewConfigLoader(workDir)
	if err != nil {
		t.Fatalf("NewConfigLoader failed: %v", err)
	}

	loader.globalPath = globalFile
	loader.projectPath = projectFile

	merged, err := loader.LoadConfigChain()
	if err != nil {
		t.Fatalf("LoadConfigChain failed: %v", err)
	}

	// Should only have one context file (same path, project overrides global)
	if len(merged.Context.AutoLoadFiles) != 1 {
		t.Errorf("expected 1 context file, got %d", len(merged.Context.AutoLoadFiles))
	}

	if merged.Context.AutoLoadFiles[0].Path != "./README.md" {
		t.Errorf("unexpected context file path: %s", merged.Context.AutoLoadFiles[0].Path)
	}

	// Project weight should override global
	if merged.Context.AutoLoadFiles[0].Weight != "high" {
		t.Errorf("expected high weight, got %s", merged.Context.AutoLoadFiles[0].Weight)
	}
}

// TestFindProjectRoot tests the project root discovery
func TestFindProjectRoot(t *testing.T) {
	workDir := t.TempDir()

	// Create go.mod in workDir
	goModPath := filepath.Join(workDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	root := findProjectRoot(workDir)
	if root != workDir {
		t.Errorf("expected root %s, got %s", workDir, root)
	}

	// Create a subdirectory and test from there
	subDir := filepath.Join(workDir, "sub", "dir")
	os.MkdirAll(subDir, 0755)

	root = findProjectRoot(subDir)
	if root != workDir {
		t.Errorf("expected root %s from subdir, got %s", workDir, root)
	}
}

// TestFindProjectRoot_NoProject tests when no project root exists
func TestFindProjectRoot_NoProject(t *testing.T) {
	tmpDir := t.TempDir()

	root := findProjectRoot(tmpDir)
	if root != "" {
		t.Errorf("expected empty string for non-project dir, got %s", root)
	}
}

// TestGetAgent tests agent lookup
func TestGetAgent(t *testing.T) {
	merged := &MergedConfig{
		Agents: []AgentRole{
			{Name: "Agent1", Model: "claude"},
			{Name: "Agent2", Model: "gemini"},
		},
	}

	agent := merged.GetAgent("Agent1")
	if agent == nil || agent.Model != "claude" {
		t.Fatal("failed to get Agent1")
	}

	agent = merged.GetAgent("NonExistent")
	if agent != nil {
		t.Fatal("expected nil for non-existent agent")
	}
}

// TestGetCommand tests command lookup
func TestGetCommand(t *testing.T) {
	merged := &MergedConfig{
		Commands: []Command{
			{Name: "/test", Handler: "test-handler"},
			{Name: "/run", Handler: "run-handler"},
		},
	}

	cmd := merged.GetCommand("/test")
	if cmd == nil || cmd.Handler != "test-handler" {
		t.Fatal("failed to get /test command")
	}

	cmd = merged.GetCommand("/nonexistent")
	if cmd != nil {
		t.Fatal("expected nil for non-existent command")
	}
}

// TestGetRoute tests route lookup
func TestGetRoute(t *testing.T) {
	merged := &MergedConfig{
		Routes: []Route{
			{Name: "FastRoute", RouteTo: "gemini"},
			{Name: "SlowRoute", RouteTo: "claude"},
		},
	}

	route := merged.GetRoute("FastRoute")
	if route == nil || route.RouteTo != "gemini" {
		t.Fatal("failed to get FastRoute")
	}

	route = merged.GetRoute("NonExistent")
	if route != nil {
		t.Fatal("expected nil for non-existent route")
	}
}

// Helper function to create a test global config
func createTestGlobalConfig(t *testing.T) (string, string) {
	globalDir := t.TempDir()
	globalFile := filepath.Join(globalDir, "FLIP2.md")

	content := `# FLIP2.md - Global Configuration

**Project:** GlobalTest
**Version:** 1.0
**Coordinator:** global-coord
**Last Updated:** 2026-01-01

---

## Agents

### Agent Role: DefaultAgent
- **ID Pattern:** ` + "`default-*`" + `
- **Model:** global-model
- **Capabilities:** ` + "`read-logs`" + `
- **Permissions:** ` + "`read-inbox`" + `
- **Max Concurrent Tasks:** 1
- **Escalation Required For:** ` + "`none`" + `
- **Cost Budget (USD/hour):** 1.00
- **Description:** Default global agent

---

## Commands

### Command: /default
- **Aliases:** ` + "`test`" + `
- **Handler:** ` + "`global-handler`" + `
- **Args:** ` + "`<arg>`" + `
- **Description:** Default command
- **Requires Approval:** no
- **Allowed Roles:** ` + "`default`" + `

---

## Routing

### Route: DefaultRoute
- **When:** ` + "`true`" + `
- **Route To:** ` + "`claude`" + `
- **Reason:** Default routing
- **Cost Impact:** ` + "`0.0`" + `
`

	if err := os.WriteFile(globalFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	return globalDir, globalFile
}

// Helper function to create a test project
func createTestProject(t *testing.T, workDir string) {
	goModPath := filepath.Join(workDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}
}

// Helper function to create a test project config
func createTestProjectConfig(t *testing.T, workDir string) string {
	projectFile := filepath.Join(workDir, "FLIP2.md")

	content := `# FLIP2.md - Project Configuration

**Project:** ProjectTest
**Version:** 2.0
**Coordinator:** project-coord
**Last Updated:** 2026-01-02

---

## Agents

### Agent Role: DefaultAgent
- **ID Pattern:** ` + "`project-*`" + `
- **Model:** project-model
- **Capabilities:** ` + "`read-logs, write-files`" + `
- **Permissions:** ` + "`read-inbox, send-signals`" + `
- **Max Concurrent Tasks:** 3
- **Escalation Required For:** ` + "`destructive`" + `
- **Cost Budget (USD/hour):** 2.50
- **Description:** Project level agent

---

## Commands

### Command: /default
- **Aliases:** ` + "`p-test`" + `
- **Handler:** ` + "`project-handler`" + `
- **Args:** ` + "`<arg1> <arg2>`" + `
- **Description:** Project command
- **Requires Approval:** yes
- **Allowed Roles:** ` + "`project`" + `

---

## Routing

### Route: DefaultRoute
- **When:** ` + "`true`" + `
- **Route To:** ` + "`gemini`" + `
- **Reason:** Project routing
- **Cost Impact:** ` + "`-0.50`" + `
`

	if err := os.WriteFile(projectFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	return projectFile
}

// Helper function to create a test directory config
func createTestDirectoryConfig(t *testing.T, workDir string) string {
	directoryFile := filepath.Join(workDir, "FLIP2.md")

	content := `# FLIP2.md - Directory Configuration

**Project:** DirectoryTest
**Version:** 3.0
**Coordinator:** directory-coord
**Last Updated:** 2026-01-03

---

## Agents

### Agent Role: DefaultAgent
- **ID Pattern:** ` + "`dir-*`" + `
- **Model:** directory-model
- **Capabilities:** ` + "`read-logs, write-files, execute`" + `
- **Permissions:** ` + "`read-inbox, send-signals, create-tasks`" + `
- **Max Concurrent Tasks:** 5
- **Escalation Required For:** ` + "`none`" + `
- **Cost Budget (USD/hour):** 5.00
- **Description:** Directory level agent

---

## Commands

### Command: /default
- **Aliases:** ` + "`d-test, dir-test`" + `
- **Handler:** ` + "`directory-handler`" + `
- **Args:** ` + "`<arg1> <arg2> <arg3>`" + `
- **Description:** Directory command
- **Requires Approval:** no
- **Allowed Roles:** ` + "`directory, admin`" + `

---

## Routing

### Route: DefaultRoute
- **When:** ` + "`true`" + `
- **Route To:** ` + "`claude`" + `
- **Reason:** Directory routing
- **Cost Impact:** ` + "`+1.0`" + `
`

	if err := os.WriteFile(directoryFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write directory config: %v", err)
	}

	return directoryFile
}

// Helper function to create global config with context
func createTestGlobalConfigWithContext(t *testing.T) (string, string) {
	globalDir := t.TempDir()
	globalFile := filepath.Join(globalDir, "FLIP2.md")

	content := `# FLIP2.md - Global Configuration

**Project:** GlobalTest
**Version:** 1.0
**Coordinator:** global-coord
**Last Updated:** 2026-01-01

---

## Agents

### Agent Role: Agent1
- **ID Pattern:** ` + "`agent1-*`" + `
- **Model:** claude
- **Capabilities:** ` + "`read-logs`" + `
- **Permissions:** ` + "`read-inbox`" + `
- **Max Concurrent Tasks:** 1
- **Escalation Required For:** ` + "`none`" + `
- **Cost Budget (USD/hour):** 1.00
- **Description:** Agent 1

---

## Commands

### Command: /cmd1
- **Aliases:** ` + "`test`" + `
- **Handler:** ` + "`handler1`" + `
- **Args:** ` + "`<arg>`" + `
- **Description:** Command 1
- **Requires Approval:** no
- **Allowed Roles:** ` + "`agent1`" + `

---

## Context

### Auto-Load Files
- ` + "`./global/README.md`" + ` - Global readme (weight: high)
`

	if err := os.WriteFile(globalFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	return globalDir, globalFile
}

// Helper function to create project config with context
func createTestProjectConfigWithContext(t *testing.T, workDir string) string {
	projectFile := filepath.Join(workDir, "FLIP2.md")

	content := `# FLIP2.md - Project Configuration

**Project:** ProjectTest
**Version:** 2.0
**Coordinator:** project-coord
**Last Updated:** 2026-01-02

---

## Agents

### Agent Role: Agent2
- **ID Pattern:** ` + "`agent2-*`" + `
- **Model:** gemini
- **Capabilities:** ` + "`read-logs, write`" + `
- **Permissions:** ` + "`read-inbox, send-signals`" + `
- **Max Concurrent Tasks:** 2
- **Escalation Required For:** ` + "`destructive`" + `
- **Cost Budget (USD/hour):** 2.50
- **Description:** Agent 2

---

## Commands

### Command: /cmd2
- **Aliases:** ` + "`p-test`" + `
- **Handler:** ` + "`handler2`" + `
- **Args:** ` + "`<arg1> <arg2>`" + `
- **Description:** Command 2
- **Requires Approval:** yes
- **Allowed Roles:** ` + "`agent2`" + `

---

## Context

### Auto-Load Files
- ` + "`./project/ARCHITECTURE.md`" + ` - Project architecture (weight: medium)
`

	if err := os.WriteFile(projectFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	return projectFile
}

// Helper function to create global config with duplicate context
func createTestGlobalConfigWithDuplicateContext(t *testing.T) (string, string) {
	globalDir := t.TempDir()
	globalFile := filepath.Join(globalDir, "FLIP2.md")

	content := `# FLIP2.md - Global Configuration

**Project:** GlobalTest
**Version:** 1.0
**Coordinator:** global-coord
**Last Updated:** 2026-01-01

---

## Agents

### Agent Role: Agent1
- **ID Pattern:** ` + "`agent1-*`" + `
- **Model:** claude
- **Capabilities:** ` + "`read-logs`" + `
- **Permissions:** ` + "`read-inbox`" + `
- **Max Concurrent Tasks:** 1
- **Escalation Required For:** ` + "`none`" + `
- **Cost Budget (USD/hour):** 1.00
- **Description:** Agent 1

---

## Commands

### Command: /cmd1
- **Aliases:** ` + "`test`" + `
- **Handler:** ` + "`handler1`" + `
- **Args:** ` + "`<arg>`" + `
- **Description:** Command 1
- **Requires Approval:** no
- **Allowed Roles:** ` + "`agent1`" + `

---

## Context

### Auto-Load Files
- ` + "`./README.md`" + ` - Readme (weight: low)
`

	if err := os.WriteFile(globalFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	return globalDir, globalFile
}

// Helper function to create project config with duplicate context
func createTestProjectConfigWithDuplicateContext(t *testing.T, workDir string) string {
	projectFile := filepath.Join(workDir, "FLIP2.md")

	content := `# FLIP2.md - Project Configuration

**Project:** ProjectTest
**Version:** 2.0
**Coordinator:** project-coord
**Last Updated:** 2026-01-02

---

## Agents

### Agent Role: Agent2
- **ID Pattern:** ` + "`agent2-*`" + `
- **Model:** gemini
- **Capabilities:** ` + "`read-logs, write`" + `
- **Permissions:** ` + "`read-inbox, send-signals`" + `
- **Max Concurrent Tasks:** 2
- **Escalation Required For:** ` + "`destructive`" + `
- **Cost Budget (USD/hour):** 2.50
- **Description:** Agent 2

---

## Commands

### Command: /cmd2
- **Aliases:** ` + "`p-test`" + `
- **Handler:** ` + "`handler2`" + `
- **Args:** ` + "`<arg1> <arg2>`" + `
- **Description:** Command 2
- **Requires Approval:** yes
- **Allowed Roles:** ` + "`agent2`" + `

---

## Context

### Auto-Load Files
- ` + "`./README.md`" + ` - Readme (weight: high)
`

	if err := os.WriteFile(projectFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	return projectFile
}
