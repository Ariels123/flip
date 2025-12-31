#!/bin/bash
# Emergency Windows Deployment Script

set -e
cd "$(dirname "$0")/.."

echo "=== Emergency Windows Deployment ==="

# 1. Create Windows config
echo "Creating Windows config..."
cat > config-win-prod.yaml << 'EOF'
flip2:
  daemon:
    pid_file: C:\flip2\flip2d.pid
    log_file: C:\flip2\flip2d.log
    log_level: info

  pocketbase:
    host: 0.0.0.0
    port: 8090
    data_dir: C:\flip2\pb_data
    tls:
      enabled: false

  security:
    api_keys_enabled: true
    api_key: flip2_secret_key_123
    jwt_secret: flip2_jwt_secret_key_456
    bootstrap_api_key: flip2_bootstrap_key_789

  sync:
    enabled: true
    node_id: windows
    sync_interval: 15s
    peers:
      - id: mac
        url: http://192.168.1.53:8090
        api_key: flip2_secret_key_123
        enabled: true

  archiver:
    enabled: true
    active_retention_days: 3
    recent_retention_days: 90
    check_interval: 6h
    batch_size: 200
    archive_path: C:\flip2\archives\signals
    active_agents:
      - claude-mac
      - claude-win
    deprecated_agents:
      - gemini
      - claude

  executor:
    max_concurrent_tasks: 3
    default_timeout: 300s

  metrics:
    enabled: true

  supervisor:
    enabled: true
    max_failures: 3
    recovery_delay: 10s
EOF

# 2. Stop Windows daemon (if running)
echo "Stopping Windows daemon..."
ssh Agnizar@192.168.1.220 'taskkill /F /IM flip2d.exe 2>nul || echo "Not running"' || true

# 3. Create directory structure on Windows
echo "Creating Windows directories..."
ssh Agnizar@192.168.1.220 'mkdir C:\flip2 2>nul || echo "Directory exists"' || true
ssh Agnizar@192.168.1.220 'mkdir C:\flip2\pb_data 2>nul || echo "pb_data exists"' || true
ssh Agnizar@192.168.1.220 'mkdir C:\flip2\archives 2>nul || echo "archives exists"' || true
ssh Agnizar@192.168.1.220 'mkdir C:\flip2\archives\signals 2>nul || echo "signals archive exists"' || true

# 4. Deploy files
echo "Deploying files to Windows..."
scp flip2d-win.exe Agnizar@192.168.1.220:C:/flip2/flip2d-new.exe
scp config-win-prod.yaml Agnizar@192.168.1.220:C:/flip2/config.yaml

# 5. Backup old binary, install new
echo "Installing new binary..."
ssh Agnizar@192.168.1.220 'copy C:\flip2\flip2d.exe C:\flip2\flip2d-backup.exe 2>nul || echo "No old binary"' || true
ssh Agnizar@192.168.1.220 'move /Y C:\flip2\flip2d-new.exe C:\flip2\flip2d.exe'

# 6. Start daemon
echo "Starting Windows daemon..."
ssh Agnizar@192.168.1.220 'cd C:\flip2 && set FLIP2_ENV=production && start /B flip2d.exe --config config.yaml'

sleep 5

# 7. Verify
echo ""
echo "Verifying Windows daemon..."
curl -s -k http://192.168.1.220:8090/api/health && echo "✓ Windows daemon healthy" || echo "✗ Windows daemon not responding"

echo ""
echo "=== Deployment Complete ==="
echo "Monitor logs: ssh Agnizar@192.168.1.220 'type C:\flip2\flip2d.log'"
