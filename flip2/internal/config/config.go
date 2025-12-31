package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the FLIP2 configuration
type Config struct {
	Flip2 struct {
		Daemon     DaemonConfig             `yaml:"daemon"`
		PocketBase PocketBaseConfig         `yaml:"pocketbase"`
		Backends   map[string]BackendConfig `yaml:"backends"`
		Scheduler  SchedulerConfig          `yaml:"scheduler"`
		Executor   ExecutorConfig           `yaml:"executor"`
		Metrics    MetricsConfig            `yaml:"metrics"`
		Security   SecurityConfig           `yaml:"security"`
		Sync       SyncConfig               `yaml:"sync"`
		Archiver   ArchiverConfig           `yaml:"archiver"`
	} `yaml:"flip2"`
}

type DaemonConfig struct {
	PIDFile          string `yaml:"pid_file"`
	LogFile          string `yaml:"log_file"`
	LogLevel         string `yaml:"log_level"`
	LogCaptureDir    string `yaml:"log_capture_dir"`
	MaxLogFileSizeMB int    `yaml:"max_log_file_size_mb"`
	MaxLogFiles      int    `yaml:"max_log_files"`
}

type PocketBaseConfig struct {
	Host     string    `yaml:"host"`
	Port     int       `yaml:"port"`
	DataDir  string    `yaml:"data_dir"`
	Database string    `yaml:"database"`
	TLS      TLSConfig `yaml:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type BackendConfig struct {
	Command   string        `yaml:"command"`
	Args      []string      `yaml:"args"`
	Timeout   time.Duration `yaml:"timeout"`
	MaxTokens int           `yaml:"max_tokens"`
	Type      string        `yaml:"type"` // "process" or "http"
	URL       string        `yaml:"url"`
}

type SchedulerConfig struct {
	Timezone          string               `yaml:"timezone"`
	MaxConcurrentJobs int                  `yaml:"max_concurrent_jobs"`
	Jobs              map[string]JobConfig `yaml:"jobs"`
}

type JobConfig struct {
	Cron    string `yaml:"cron"`
	Enabled bool   `yaml:"enabled"`
}

type ExecutorConfig struct {
	MaxConcurrentTasks int           `yaml:"max_concurrent_tasks"`
	DefaultTimeout     time.Duration `yaml:"default_timeout"`
	RetryAttempts      int           `yaml:"retry_attempts"`
	RetryDelay         time.Duration `yaml:"retry_delay"`
	WorkerPrefix       string        `yaml:"worker_prefix"`
}

type MetricsConfig struct {
	Enabled       bool `yaml:"enabled"`
	RetentionDays int  `yaml:"retention_days"`
}

type SecurityConfig struct {
	AdminEmail      string `yaml:"admin_email"`
	APIKeysEnabled  bool   `yaml:"api_keys_enabled"`
	APIKey          string `yaml:"api_key"`
	JWTSecret       string `yaml:"jwt_secret"`
	BootstrapAPIKey string `yaml:"bootstrap_api_key"` // JWT token generation auth
}

// SyncConfig configures peer-to-peer synchronization
type SyncConfig struct {
	Enabled       bool             `yaml:"enabled"`
	NodeID        string           `yaml:"node_id"`
	SyncInterval  time.Duration    `yaml:"sync_interval"`
	Peers         []PeerConfig     `yaml:"peers"`
}

// PeerConfig defines a remote sync peer
type PeerConfig struct {
	ID      string `yaml:"id"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key"`
	Enabled bool   `yaml:"enabled"`
}

// ArchiverConfig configures the message archiver
type ArchiverConfig struct {
	Enabled             bool          `yaml:"enabled"`
	ActiveRetentionDays int           `yaml:"active_retention_days"`
	RecentRetentionDays int           `yaml:"recent_retention_days"`
	CheckInterval       time.Duration `yaml:"check_interval"`
	BatchSize           int           `yaml:"batch_size"`
	ActiveAgents        []string      `yaml:"active_agents"`
	DeprecatedAgents    []string      `yaml:"deprecated_agents"`
	ArchivePath         string        `yaml:"archive_path"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults if needed
	if config.Flip2.PocketBase.Port == 0 {
		config.Flip2.PocketBase.Port = 8090
	}
	if config.Flip2.PocketBase.DataDir == "" {
		config.Flip2.PocketBase.DataDir = "./pb_data"
	}

	// Sync defaults
	if config.Flip2.Sync.SyncInterval == 0 {
		config.Flip2.Sync.SyncInterval = 30 * time.Second
	}
	if config.Flip2.Sync.NodeID == "" {
		hostname, _ := os.Hostname()
		config.Flip2.Sync.NodeID = hostname
	}

	return &config, nil
}
