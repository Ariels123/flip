# Windows Deployment Guide (Manual)

**Status:** Automated deployment blocked by SSH authentication issues
**Last Updated:** 2025-12-31

---

## Issue

All SSH authentication methods to Windows (192.168.1.220) are failing:
- SSH key authentication: Keys are encrypted or not accepted
- Password authentication: Cannot prompt in non-interactive context
- SCP file transfer: Permission denied

---

## Manual Deployment Steps

### Prerequisites

1. Have access to Windows PC at 192.168.1.220
2. Files ready on Mac:
   - Binary: `/Users/arielspivakovsky/src/flip/flip2/flip2d-win.exe` (35MB)
   - Config: `/Users/arielspivakovsky/src/flip/flip2/config-win-prod.yaml`

### Step 1: Transfer Files to Windows

**Option A: Using File Sharing/Network Drive**
```powershell
# On Windows, create directory
mkdir C:\flip2
mkdir C:\flip2\pb_data
mkdir C:\flip2\archives\signals
```

Then manually copy:
- `flip2d-win.exe` → `C:\flip2\flip2d.exe`
- `config-win-prod.yaml` → `C:\flip2\config.yaml`

**Option B: Using USB Drive**
1. Copy files to USB on Mac
2. Plug USB into Windows
3. Copy to `C:\flip2\`

**Option C: Using Remote Desktop**
1. Connect via RDP to 192.168.1.220
2. Open browser, download from Mac via HTTP
3. Or use shared clipboard to transfer

### Step 2: Verify Files on Windows

Open PowerShell and run:
```powershell
cd C:\flip2
dir

# Should see:
# - flip2d.exe
# - config.yaml
# - pb_data\ (directory)
# - archives\ (directory)
```

### Step 3: Stop Old Daemon (if running)

```powershell
taskkill /F /IM flip2d.exe
```

### Step 4: Set Environment and Start Daemon

```powershell
cd C:\flip2
set FLIP2_ENV=production
start /B flip2d.exe --config config.yaml
```

### Step 5: Verify Daemon is Running

Wait 5 seconds, then:

```powershell
# Check process
tasklist | findstr flip2d

# Check health endpoint (in browser or curl)
curl http://localhost:8090/api/health
```

Expected response:
```json
{"message":"API is healthy.","code":200,"data":{}}
```

### Step 6: Verify from Mac

From Mac terminal:
```bash
curl http://192.168.1.220:8090/api/health
```

Should return same healthy response.

---

## Troubleshooting

### Daemon Won't Start

Check the log file:
```powershell
type C:\flip2\flip2d.log
```

Common issues:
1. **Port 8090 already in use:** Stop other services
2. **pb_data missing:** Create `mkdir C:\flip2\pb_data`
3. **Permission denied:** Run PowerShell as Administrator

### Health Check Fails

1. **Check firewall:** 
   ```powershell
   netsh advfirewall firewall add rule name="FLIP2" dir=in action=allow protocol=TCP localport=8090
   ```

2. **Check process is running:**
   ```powershell
   tasklist | findstr flip2d
   netstat -ano | findstr :8090
   ```

3. **Check logs:**
   ```powershell
   type C:\flip2\flip2d.log | more
   ```

### Sync Not Working

1. **Verify Mac can reach Windows:**
   ```bash
   # From Mac
   curl http://192.168.1.220:8090/api/health
   ```

2. **Verify Windows can reach Mac:**
   ```powershell
   # From Windows
   curl http://192.168.1.53:8090/api/health
   ```

3. **Check API keys match** in both configs:
   - Mac: `config/config.yaml`
   - Windows: `C:\flip2\config.yaml`
   - Should both have: `api_key: flip2_secret_key_123`

---

## Configuration File (config.yaml)

The Windows config should be:

```yaml
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

  scheduler:
    timezone: UTC
    max_concurrent_jobs: 4
```

---

## Testing Sync

### Send Signal from Mac to Windows

```bash
# From Mac
curl -X POST https://localhost:8090/api/collections/signals/records \
  -H "Content-Type: application/json" \
  -H "X-API-Key: flip2_secret_key_123" \
  -d '{
    "signal_id": "test-mac-to-win-001",
    "from_agent": "mac",
    "to_agent": "windows",
    "signal_type": "message",
    "priority": "normal",
    "content": "Test sync from Mac to Windows"
  }' -k
```

Wait 15-30 seconds for sync, then check on Windows:
```powershell
curl "http://localhost:8090/api/collections/signals/records?filter=signal_id='test-mac-to-win-001'" -H "X-API-Key: flip2_secret_key_123"
```

### Send Signal from Windows to Mac

```powershell
# From Windows
curl -X POST http://localhost:8090/api/collections/signals/records `
  -H "Content-Type: application/json" `
  -H "X-API-Key: flip2_secret_key_123" `
  -d '{\"signal_id\":\"test-win-to-mac-001\",\"from_agent\":\"windows\",\"to_agent\":\"mac\",\"signal_type\":\"message\",\"priority\":\"normal\",\"content\":\"Test sync from Windows to Mac\"}'
```

Wait 15-30 seconds, then check on Mac:
```bash
curl -s -k "https://localhost:8090/api/collections/signals/records?filter=signal_id='test-win-to-mac-001'" \
  -H "X-API-Key: flip2_secret_key_123"
```

---

## Automated Deployment (When SSH Works)

Once SSH authentication is fixed, use:
```bash
# From Mac
python3 scripts/deploy_windows.py
```

Or with password:
```bash
export WINDOWS_PASSWORD='your_password'
python3 scripts/deploy_windows.py
```

---

## Next Steps

1. Manually deploy using steps above
2. Verify health endpoints on both Mac and Windows
3. Test bidirectional sync
4. Fix SSH authentication for future automated deployments
5. Add monitoring for sync failures

---

**Files Ready for Deployment:**
- Binary: `flip2d-win.exe` (35.1 MB) ✓
- Config: `config-win-prod.yaml` ✓
- Python script: `scripts/deploy_windows.py` (for when SSH works)
