package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadConfigYAML tests loading and parsing YAML configuration files
func TestLoadConfigYAML(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantErr   bool
		errSubstr string
		validate  func(*testing.T, *Config)
	}{
		{
			name: "valid FLIP2 config",
			content: `flip2:
  daemon:
    pid_file: /var/run/flip2.pid
    log_file: /var/log/flip2.log
    log_level: info
    max_log_file_size_mb: 100
    max_log_files: 10
  pocketbase:
    host: localhost
    port: 8090
    data_dir: ./pb_data
    database: pb.db
    tls:
      enabled: false
  backends:
    claude:
      command: claude
      timeout: 30s
      max_tokens: 4096
      type: http
      url: http://localhost:9000
    gemini:
      command: gemini
      timeout: 45s
      max_tokens: 8000
      type: http
      url: http://localhost:9001
  scheduler:
    timezone: UTC
    max_concurrent_jobs: 5
    jobs:
      sync:
        cron: "0 */6 * * *"
        enabled: true
  executor:
    max_concurrent_tasks: 10
    default_timeout: 5m
    retry_attempts: 3
    retry_delay: 1s
    worker_prefix: "worker-"
  metrics:
    enabled: true
    retention_days: 30
  security:
    admin_email: admin@example.com
    api_keys_enabled: true
    api_key: test-key-12345
    jwt_secret: secret-key-abcdef
    bootstrap_api_key: bootstrap-key
  sync:
    enabled: true
    node_id: node-1
    sync_interval: 30s
    peers:
      - id: node-2
        url: http://node2:8090
        api_key: api-key-2
        enabled: true
  archiver:
    enabled: true
    active_retention_days: 90
    recent_retention_days: 7
    check_interval: 1h
    batch_size: 1000
    active_agents:
      - agent1
      - agent2
    deprecated_agents:
      - old-agent
    archive_path: ./archives
  commmonitor:
    enabled: true
    threshold: 0.75
    valid_agents:
      - agent1
      - agent2
    typo_corrections:
      agnet: agent1
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Flip2.Daemon.LogLevel != "info" {
					t.Errorf("LogLevel = %s, want info", cfg.Flip2.Daemon.LogLevel)
				}
				if cfg.Flip2.PocketBase.Port != 8090 {
					t.Errorf("PocketBase.Port = %d, want 8090", cfg.Flip2.PocketBase.Port)
				}
				if len(cfg.Flip2.Backends) != 2 {
					t.Errorf("Backends count = %d, want 2", len(cfg.Flip2.Backends))
				}
				if cfg.Flip2.Executor.MaxConcurrentTasks != 10 {
					t.Errorf("MaxConcurrentTasks = %d, want 10", cfg.Flip2.Executor.MaxConcurrentTasks)
				}
				if cfg.Flip2.CommMonitor.Threshold != 0.75 {
					t.Errorf("CommMonitor.Threshold = %f, want 0.75", cfg.Flip2.CommMonitor.Threshold)
				}
			},
		},
		{
			name: "minimal valid config",
			content: `flip2:
  pocketbase:
    host: localhost
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				// Check defaults are applied
				if cfg.Flip2.PocketBase.Port != 8090 {
					t.Errorf("Default PocketBase.Port = %d, want 8090", cfg.Flip2.PocketBase.Port)
				}
				if cfg.Flip2.PocketBase.DataDir != "./pb_data" {
					t.Errorf("Default DataDir = %s, want ./pb_data", cfg.Flip2.PocketBase.DataDir)
				}
				if cfg.Flip2.Sync.SyncInterval != 30*time.Second {
					t.Errorf("Default SyncInterval = %v, want 30s", cfg.Flip2.Sync.SyncInterval)
				}
				if cfg.Flip2.CommMonitor.Threshold != 0.75 {
					t.Errorf("Default CommMonitor.Threshold = %f, want 0.75", cfg.Flip2.CommMonitor.Threshold)
				}
			},
		},
		{
			name: "invalid YAML syntax",
			content: `flip2:
  daemon:
    log_level: [unclosed bracket`,
			wantErr:   true,
			errSubstr: "parse config",
		},
		{
			name: "empty backends",
			content: `flip2:
  pocketbase:
    host: localhost
  backends: {}
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Flip2.Backends) != 0 {
					t.Errorf("Expected empty backends, got %d", len(cfg.Flip2.Backends))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if (err == nil) != !tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errSubstr != "" && err != nil {
				if !contains(err.Error(), tt.errSubstr) {
					t.Errorf("LoadConfig() error = %q, want containing %q", err.Error(), tt.errSubstr)
				}
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

// TestConfigDefaults tests that default values are properly applied
func TestConfigDefaults(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *Config)
	}{
		{
			name: "pocketbase defaults",
			content: `flip2:
  pocketbase:
    host: localhost
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Flip2.PocketBase.Port != 8090 {
					t.Errorf("Port = %d, want 8090", cfg.Flip2.PocketBase.Port)
				}
				if cfg.Flip2.PocketBase.DataDir != "./pb_data" {
					t.Errorf("DataDir = %s, want ./pb_data", cfg.Flip2.PocketBase.DataDir)
				}
			},
		},
		{
			name: "sync defaults",
			content: `flip2:
  pocketbase:
    host: localhost
  sync:
    enabled: true
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Flip2.Sync.SyncInterval != 30*time.Second {
					t.Errorf("SyncInterval = %v, want 30s", cfg.Flip2.Sync.SyncInterval)
				}
				hostname, _ := os.Hostname()
				if cfg.Flip2.Sync.NodeID != hostname {
					t.Errorf("NodeID = %s, want %s", cfg.Flip2.Sync.NodeID, hostname)
				}
			},
		},
		{
			name: "commmonitor defaults",
			content: `flip2:
  pocketbase:
    host: localhost
  commmonitor:
    enabled: true
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Flip2.CommMonitor.Threshold != 0.75 {
					t.Errorf("Threshold = %f, want 0.75", cfg.Flip2.CommMonitor.Threshold)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			tt.validate(t, cfg)
		})
	}
}

// TestCommMonitorValidation tests CommMonitor configuration validation
func TestCommMonitorValidation(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid typo corrections",
			content: `flip2:
  pocketbase:
    host: localhost
  commmonitor:
    enabled: true
    valid_agents:
      - agent1
      - agent2
    typo_corrections:
      agnet: agent1
      agetnt: agent2
`,
			wantErr: false,
		},
		{
			name: "invalid typo correction target",
			content: `flip2:
  pocketbase:
    host: localhost
  commmonitor:
    enabled: true
    valid_agents:
      - agent1
    typo_corrections:
      agnet: invalid-agent
`,
			wantErr:   true,
			errSubstr: "not in valid_agents list",
		},
		{
			name: "disabled commmonitor skips validation",
			content: `flip2:
  pocketbase:
    host: localhost
  commmonitor:
    enabled: false
    typo_corrections:
      agnet: nonexistent
`,
			wantErr: false,
		},
		{
			name: "no typo corrections",
			content: `flip2:
  pocketbase:
    host: localhost
  commmonitor:
    enabled: true
    valid_agents:
      - agent1
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			_, err := LoadConfig(configPath)
			if (err == nil) != !tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errSubstr != "" && err != nil {
				if !contains(err.Error(), tt.errSubstr) {
					t.Errorf("LoadConfig() error = %q, want containing %q", err.Error(), tt.errSubstr)
				}
			}
		})
	}
}

// TestBackendConfiguration tests backend configuration loading
func TestBackendConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *Config)
	}{
		{
			name: "multiple backends",
			content: `flip2:
  pocketbase:
    host: localhost
  backends:
    claude:
      command: claude
      timeout: 30s
      max_tokens: 4096
      type: http
      url: http://localhost:9000
    gemini:
      command: gemini
      args:
        - --model=gemini-pro
      timeout: 45s
      max_tokens: 8000
      type: http
      url: http://localhost:9001
    local:
      command: /usr/bin/llm
      args:
        - --stream
      timeout: 10s
      max_tokens: 2048
      type: process
`,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Flip2.Backends) != 3 {
					t.Errorf("Backend count = %d, want 3", len(cfg.Flip2.Backends))
					return
				}

				claude := cfg.Flip2.Backends["claude"]
				if claude.Type != "http" {
					t.Errorf("Claude type = %s, want http", claude.Type)
				}
				if claude.URL != "http://localhost:9000" {
					t.Errorf("Claude URL = %s, want http://localhost:9000", claude.URL)
				}
				if claude.Timeout != 30*time.Second {
					t.Errorf("Claude timeout = %v, want 30s", claude.Timeout)
				}

				gemini := cfg.Flip2.Backends["gemini"]
				if len(gemini.Args) != 1 {
					t.Errorf("Gemini args = %d, want 1", len(gemini.Args))
				}

				local := cfg.Flip2.Backends["local"]
				if local.Type != "process" {
					t.Errorf("Local type = %s, want process", local.Type)
				}
				if local.URL != "" {
					t.Errorf("Local URL = %s, want empty", local.URL)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			tt.validate(t, cfg)
		})
	}
}

// TestSchedulerConfiguration tests scheduler job configuration
func TestSchedulerConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *Config)
	}{
		{
			name: "multiple scheduled jobs",
			content: `flip2:
  pocketbase:
    host: localhost
  scheduler:
    timezone: America/New_York
    max_concurrent_jobs: 5
    jobs:
      sync:
        cron: "0 */6 * * *"
        enabled: true
      cleanup:
        cron: "0 3 * * *"
        enabled: true
      report:
        cron: "0 9 * * MON"
        enabled: false
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Flip2.Scheduler.Timezone != "America/New_York" {
					t.Errorf("Timezone = %s, want America/New_York", cfg.Flip2.Scheduler.Timezone)
				}
				if cfg.Flip2.Scheduler.MaxConcurrentJobs != 5 {
					t.Errorf("MaxConcurrentJobs = %d, want 5", cfg.Flip2.Scheduler.MaxConcurrentJobs)
				}
				if len(cfg.Flip2.Scheduler.Jobs) != 3 {
					t.Errorf("Jobs count = %d, want 3", len(cfg.Flip2.Scheduler.Jobs))
				}

				syncJob := cfg.Flip2.Scheduler.Jobs["sync"]
				if syncJob.Cron != "0 */6 * * *" {
					t.Errorf("Sync cron = %s, want 0 */6 * * *", syncJob.Cron)
				}
				if !syncJob.Enabled {
					t.Errorf("Sync enabled = %v, want true", syncJob.Enabled)
				}

				reportJob := cfg.Flip2.Scheduler.Jobs["report"]
				if reportJob.Enabled {
					t.Errorf("Report enabled = %v, want false", reportJob.Enabled)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			tt.validate(t, cfg)
		})
	}
}

// TestExecutorConfiguration tests executor task configuration
func TestExecutorConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *Config)
	}{
		{
			name: "executor with retry settings",
			content: `flip2:
  pocketbase:
    host: localhost
  executor:
    max_concurrent_tasks: 20
    default_timeout: 10m
    retry_attempts: 5
    retry_delay: 2s
    worker_prefix: "exec-"
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Flip2.Executor.MaxConcurrentTasks != 20 {
					t.Errorf("MaxConcurrentTasks = %d, want 20", cfg.Flip2.Executor.MaxConcurrentTasks)
				}
				if cfg.Flip2.Executor.DefaultTimeout != 10*time.Minute {
					t.Errorf("DefaultTimeout = %v, want 10m", cfg.Flip2.Executor.DefaultTimeout)
				}
				if cfg.Flip2.Executor.RetryAttempts != 5 {
					t.Errorf("RetryAttempts = %d, want 5", cfg.Flip2.Executor.RetryAttempts)
				}
				if cfg.Flip2.Executor.RetryDelay != 2*time.Second {
					t.Errorf("RetryDelay = %v, want 2s", cfg.Flip2.Executor.RetryDelay)
				}
				if cfg.Flip2.Executor.WorkerPrefix != "exec-" {
					t.Errorf("WorkerPrefix = %s, want exec-", cfg.Flip2.Executor.WorkerPrefix)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			tt.validate(t, cfg)
		})
	}
}

// TestArchiverConfiguration tests archiver configuration
func TestArchiverConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *Config)
	}{
		{
			name: "archiver with retention policies",
			content: `flip2:
  pocketbase:
    host: localhost
  archiver:
    enabled: true
    active_retention_days: 90
    recent_retention_days: 7
    check_interval: 1h
    batch_size: 1000
    active_agents:
      - agent1
      - agent2
      - agent3
    deprecated_agents:
      - old-agent-1
      - old-agent-2
    archive_path: /var/flip2/archives
`,
			validate: func(t *testing.T, cfg *Config) {
				if !cfg.Flip2.Archiver.Enabled {
					t.Errorf("Enabled = %v, want true", cfg.Flip2.Archiver.Enabled)
				}
				if cfg.Flip2.Archiver.ActiveRetentionDays != 90 {
					t.Errorf("ActiveRetentionDays = %d, want 90", cfg.Flip2.Archiver.ActiveRetentionDays)
				}
				if cfg.Flip2.Archiver.CheckInterval != 1*time.Hour {
					t.Errorf("CheckInterval = %v, want 1h", cfg.Flip2.Archiver.CheckInterval)
				}
				if len(cfg.Flip2.Archiver.ActiveAgents) != 3 {
					t.Errorf("ActiveAgents count = %d, want 3", len(cfg.Flip2.Archiver.ActiveAgents))
				}
				if len(cfg.Flip2.Archiver.DeprecatedAgents) != 2 {
					t.Errorf("DeprecatedAgents count = %d, want 2", len(cfg.Flip2.Archiver.DeprecatedAgents))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			tt.validate(t, cfg)
		})
	}
}

// TestSyncPeerConfiguration tests peer configuration for sync
func TestSyncPeerConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *Config)
	}{
		{
			name: "multiple sync peers",
			content: `flip2:
  pocketbase:
    host: localhost
  sync:
    enabled: true
    node_id: primary-node
    sync_interval: 60s
    peers:
      - id: secondary-1
        url: http://secondary1:8090
        api_key: key-sec1
        enabled: true
      - id: secondary-2
        url: http://secondary2:8090
        api_key: key-sec2
        enabled: true
      - id: backup
        url: http://backup:8090
        api_key: key-backup
        enabled: false
`,
			validate: func(t *testing.T, cfg *Config) {
				if !cfg.Flip2.Sync.Enabled {
					t.Errorf("Sync.Enabled = %v, want true", cfg.Flip2.Sync.Enabled)
				}
				if cfg.Flip2.Sync.NodeID != "primary-node" {
					t.Errorf("NodeID = %s, want primary-node", cfg.Flip2.Sync.NodeID)
				}
				if len(cfg.Flip2.Sync.Peers) != 3 {
					t.Errorf("Peers count = %d, want 3", len(cfg.Flip2.Sync.Peers))
				}

				// Check first peer
				if cfg.Flip2.Sync.Peers[0].ID != "secondary-1" {
					t.Errorf("First peer ID = %s, want secondary-1", cfg.Flip2.Sync.Peers[0].ID)
				}
				if !cfg.Flip2.Sync.Peers[0].Enabled {
					t.Errorf("First peer enabled = %v, want true", cfg.Flip2.Sync.Peers[0].Enabled)
				}

				// Check last peer
				if cfg.Flip2.Sync.Peers[2].Enabled {
					t.Errorf("Backup peer enabled = %v, want false", cfg.Flip2.Sync.Peers[2].Enabled)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			tt.validate(t, cfg)
		})
	}
}

// TestSecurityConfiguration tests security settings
func TestSecurityConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *Config)
	}{
		{
			name: "full security config",
			content: `flip2:
  pocketbase:
    host: localhost
  security:
    admin_email: admin@example.com
    api_keys_enabled: true
    api_key: pk_test_12345abcde
    jwt_secret: jwt_secret_key
    bootstrap_api_key: bootstrap_key_xyz
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Flip2.Security.AdminEmail != "admin@example.com" {
					t.Errorf("AdminEmail = %s, want admin@example.com", cfg.Flip2.Security.AdminEmail)
				}
				if !cfg.Flip2.Security.APIKeysEnabled {
					t.Errorf("APIKeysEnabled = %v, want true", cfg.Flip2.Security.APIKeysEnabled)
				}
				if cfg.Flip2.Security.APIKey != "pk_test_12345abcde" {
					t.Errorf("APIKey mismatch")
				}
				if cfg.Flip2.Security.JWTSecret != "jwt_secret_key" {
					t.Errorf("JWTSecret mismatch")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			tt.validate(t, cfg)
		})
	}
}

// TestTLSConfiguration tests TLS/SSL settings
func TestTLSConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *Config)
	}{
		{
			name: "TLS enabled",
			content: `flip2:
  pocketbase:
    host: localhost
    tls:
      enabled: true
      cert_file: /etc/flip2/certs/server.crt
      key_file: /etc/flip2/certs/server.key
`,
			validate: func(t *testing.T, cfg *Config) {
				if !cfg.Flip2.PocketBase.TLS.Enabled {
					t.Errorf("TLS.Enabled = %v, want true", cfg.Flip2.PocketBase.TLS.Enabled)
				}
				if cfg.Flip2.PocketBase.TLS.CertFile != "/etc/flip2/certs/server.crt" {
					t.Errorf("TLS.CertFile mismatch")
				}
				if cfg.Flip2.PocketBase.TLS.KeyFile != "/etc/flip2/certs/server.key" {
					t.Errorf("TLS.KeyFile mismatch")
				}
			},
		},
		{
			name: "TLS disabled",
			content: `flip2:
  pocketbase:
    host: localhost
    tls:
      enabled: false
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Flip2.PocketBase.TLS.Enabled {
					t.Errorf("TLS.Enabled = %v, want false", cfg.Flip2.PocketBase.TLS.Enabled)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			tt.validate(t, cfg)
		})
	}
}

// TestMetricsConfiguration tests metrics settings
func TestMetricsConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *Config)
	}{
		{
			name: "metrics enabled",
			content: `flip2:
  pocketbase:
    host: localhost
  metrics:
    enabled: true
    retention_days: 30
`,
			validate: func(t *testing.T, cfg *Config) {
				if !cfg.Flip2.Metrics.Enabled {
					t.Errorf("Metrics.Enabled = %v, want true", cfg.Flip2.Metrics.Enabled)
				}
				if cfg.Flip2.Metrics.RetentionDays != 30 {
					t.Errorf("RetentionDays = %d, want 30", cfg.Flip2.Metrics.RetentionDays)
				}
			},
		},
		{
			name: "metrics disabled",
			content: `flip2:
  pocketbase:
    host: localhost
  metrics:
    enabled: false
    retention_days: 0
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Flip2.Metrics.Enabled {
					t.Errorf("Metrics.Enabled = %v, want false", cfg.Flip2.Metrics.Enabled)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			tt.validate(t, cfg)
		})
	}
}

// TestDaemonConfiguration tests daemon logging settings
func TestDaemonConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		validate func(*testing.T, *Config)
	}{
		{
			name: "daemon with logging config",
			content: `flip2:
  pocketbase:
    host: localhost
  daemon:
    pid_file: /var/run/flip2.pid
    log_file: /var/log/flip2/flip2.log
    log_level: debug
    log_capture_dir: /var/log/flip2/capture
    max_log_file_size_mb: 50
    max_log_files: 5
`,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Flip2.Daemon.PIDFile != "/var/run/flip2.pid" {
					t.Errorf("PIDFile = %s, want /var/run/flip2.pid", cfg.Flip2.Daemon.PIDFile)
				}
				if cfg.Flip2.Daemon.LogLevel != "debug" {
					t.Errorf("LogLevel = %s, want debug", cfg.Flip2.Daemon.LogLevel)
				}
				if cfg.Flip2.Daemon.MaxLogFileSizeMB != 50 {
					t.Errorf("MaxLogFileSizeMB = %d, want 50", cfg.Flip2.Daemon.MaxLogFileSizeMB)
				}
				if cfg.Flip2.Daemon.MaxLogFiles != 5 {
					t.Errorf("MaxLogFiles = %d, want 5", cfg.Flip2.Daemon.MaxLogFiles)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() failed: %v", err)
			}

			tt.validate(t, cfg)
		})
	}
}

// TestComplexInheritanceScenario tests a realistic configuration hierarchy
func TestComplexInheritanceScenario(t *testing.T) {
	// This test validates a complete configuration as would be loaded in production
	content := `flip2:
  daemon:
    pid_file: /var/run/flip2.pid
    log_file: /var/log/flip2.log
    log_level: info
    max_log_file_size_mb: 100
    max_log_files: 10
  pocketbase:
    host: 0.0.0.0
    port: 8090
    data_dir: /data/flip2/pb_data
    database: flip2.db
    tls:
      enabled: true
      cert_file: /etc/flip2/certs/server.crt
      key_file: /etc/flip2/certs/server.key
  backends:
    claude:
      command: claude
      timeout: 30s
      max_tokens: 4096
      type: http
      url: http://localhost:9000
  scheduler:
    timezone: UTC
    max_concurrent_jobs: 5
  executor:
    max_concurrent_tasks: 10
    default_timeout: 5m
    retry_attempts: 3
    retry_delay: 1s
    worker_prefix: "worker-"
  metrics:
    enabled: true
    retention_days: 30
  security:
    admin_email: admin@example.com
    api_keys_enabled: true
  sync:
    enabled: true
  archiver:
    enabled: true
  commmonitor:
    enabled: true
    valid_agents:
      - claude
      - gemini
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify complete configuration
	if cfg.Flip2.Daemon.LogLevel != "info" {
		t.Error("daemon config not loaded")
	}
	if cfg.Flip2.PocketBase.Port != 8090 {
		t.Error("pocketbase config not loaded")
	}
	if len(cfg.Flip2.Backends) == 0 {
		t.Error("backends not loaded")
	}
	if cfg.Flip2.Scheduler.MaxConcurrentJobs != 5 {
		t.Error("scheduler config not loaded")
	}
	if cfg.Flip2.Executor.MaxConcurrentTasks != 10 {
		t.Error("executor config not loaded")
	}
	if !cfg.Flip2.Metrics.Enabled {
		t.Error("metrics not loaded")
	}
	if cfg.Flip2.Security.AdminEmail == "" {
		t.Error("security config not loaded")
	}
	if !cfg.Flip2.Sync.Enabled {
		t.Error("sync not enabled")
	}
	if !cfg.Flip2.Archiver.Enabled {
		t.Error("archiver not enabled")
	}
	if !cfg.Flip2.CommMonitor.Enabled {
		t.Error("commmonitor not enabled")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
