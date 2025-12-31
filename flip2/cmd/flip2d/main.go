// FLIP2 Daemon - Multi-Agent Coordination Service
package main

import (
	"log"
	"os"
	"path/filepath"

	"flip2/internal/daemon"

	"github.com/spf13/cobra"
)

func main() {
	var configPath string
	var foreground bool
	var pidFile string

	rootCmd := &cobra.Command{
		Use:   "flip2d",
		Short: "FLIP2 Daemon",
		Run: func(cmd *cobra.Command, args []string) {
			// Clean up config path
			if configPath == "" {
				// Try default locations
				// 1. /etc/flip2/config.yaml
				// 2. ./config/config.yaml
				if _, err := os.Stat("/etc/flip2/config.yaml"); err == nil {
					configPath = "/etc/flip2/config.yaml"
				} else if _, err := os.Stat("./config/config.yaml"); err == nil {
					configPath = "./config/config.yaml"
				} else {
					log.Fatal("No config file found. Please provide --config")
				}
			}
			
			absConfigPath, err := filepath.Abs(configPath)
			if err != nil {
				log.Fatalf("Invalid config path: %v", err)
			}

			// Set foreground mode via environment variable if flag is set
			if foreground {
				os.Setenv("FLIP2_FOREGROUND", "1")
			}

			// Initialize daemon
			d, err := daemon.New(absConfigPath)
			if err != nil {
				log.Fatalf("Failed to create daemon: %v", err)
			}

			// Start daemon (blocks)
			if err := d.Start(); err != nil {
				log.Fatalf("Daemon error: %v", err)
			}
		},
	}

	rootCmd.Flags().StringVar(&configPath, "config", "", "Path to configuration file")
	rootCmd.Flags().BoolVar(&foreground, "foreground", false, "Run in foreground (don't daemonize)")
	rootCmd.Flags().StringVar(&pidFile, "pid-file", "", "Path to PID file (overrides config)")
	
	// Pass signals to daemon? The daemon handles signals.
	
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

