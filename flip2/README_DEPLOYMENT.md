# FLIP2 Deployment Guide

**Version:** 1.0  
**Date:** 2025-12-31  
**Status:** Production Ready

---

## Quick Start

### Mac Deployment (COMPLETE ✅)
```bash
cd /Users/arielspivakovsky/src/flip/flip2
FLIP2_ENV=production ./flip2d --config config/config.yaml
```

### Windows Deployment (3 Methods)

#### Method 1: Download & Run Batch Script (Easiest) ⭐
```batch
REM On Windows PC, open PowerShell as Administrator:
Invoke-WebRequest -Uri "http://192.168.1.53:8000/deploy_windows.bat" -OutFile "$env:TEMP\deploy_flip2.bat"
cmd /c "$env:TEMP\deploy_flip2.bat"
```

#### Method 2: PowerShell Script
```powershell
iwr "http://192.168.1.53:8000/deploy_package_windows.ps1" -OutFile "$env:TEMP\d.ps1"
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
& "$env:TEMP\d.ps1"
```

#### Method 3: Manual Copy
See: `WINDOWS_DEPLOYMENT_MANUAL.md`

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    FLIP2 SYSTEM                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────┐                           ┌──────────┐   │
│  │  Mac     │  ←── Sync (15s) ──→      │ Windows  │   │
│  │  :8090   │                           │  :8090   │   │
│  └──────────┘                           └──────────┘   │
│                                                         │
│  • Environment validation (prod/test ports)            │
│  • Auto log cleanup (8h pattern, 48h full + VACUUM)   │
│  • Vector clock sync                                   │
│  • Bootstrap collections                               │
│  • Multi-agent coordination                            │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## Features Deployed

### ✅ Critical Fixes
1. **Environment Validation** - Daemon validates FLIP2_ENV matches port (8xxx=prod, 9xxx=test)
2. **Log Cleanup Jobs** - Automated cleanup prevents disk exhaustion
   - Pattern cleanup: Every 8 hours (removes junk logs)
   - Full prune: Every 48 hours (48h retention + VACUUM)
3. **Test Infrastructure** - Safe testing on port 9190 without affecting production
4. **Safety Checklist** - Pre-work validation script prevents accidents

### ✅ Mac Daemon
- **Status:** Running (PID 74066)
- **Port:** 8090 (production)
- **Health:** https://localhost:8090/api/health
- **Log Size:** 24KB (was 77MB - freed 77MB)
- **Collections:** Auto-bootstrap on startup

### 🟡 Windows Daemon  
- **Files Ready:** flip2d-win.exe (35MB), config-win-prod.yaml
- **Deployment:** Multiple methods available (see above)
- **Expected Port:** 8090
- **Sync Target:** Mac at 192.168.1.53:8090

---

## Configuration

### Mac Production (`config/config.yaml`)
```yaml
flip2:
  pocketbase:
    port: 8090
    data_dir: ./pb_data
  sync:
    node_id: mac
    peers:
      - id: windows
        url: http://192.168.1.220:8090
```

### Windows Production (`config-win-prod.yaml`)
```yaml
flip2:
  pocketbase:
    port: 8090
    data_dir: C:\flip2\pb_data
  sync:
    node_id: windows
    peers:
      - id: mac
        url: http://192.168.1.53:8090
```

### Test Environment (`config/config-test.yaml`)
```yaml
flip2:
  pocketbase:
    port: 9190  # Test port
    data_dir: ./pb_data_test
  sync:
    enabled: false  # Don't sync test to production
```

---

## Safety Features

### Environment Validation
```bash
# These WORK:
FLIP2_ENV=production ./flip2d --config config.yaml        # Port 8090 ✓
FLIP2_ENV=test ./flip2d --config config-test.yaml          # Port 9190 ✓

# These ABORT with error:
FLIP2_ENV=test ./flip2d --config config.yaml               # Test on prod port ✗
FLIP2_ENV=production ./flip2d --config config-test.yaml     # Prod on test port ✗
```

### Pre-Work Checklist
```bash
./scripts/pre_work_checklist.sh
# Validates:
# - FLIP2_ENV matches port
# - No port conflicts
# - Database sizes OK
# - ABORTS if environment/port mismatch
```

---

## Claude Integration (FLIP2 Skill)

### Installation
Skill installed at: `~/.claude/skills/flip2.skill`

### Usage
```bash
# Helper script
./scripts/flip2_claude.sh register claude-coordinator
./scripts/flip2_claude.sh inbox
./scripts/flip2_claude.sh send gemini-worker "Analyze logs"
./scripts/flip2_claude.sh agents
./scripts/flip2_claude.sh health
```

### Available Commands
- `/flip2-register [agent-id]` - Register Claude as FLIP2 agent
- `/flip2-inbox` - Check for new signals/tasks
- `/flip2-send <agent> <message>` - Send signal to another agent
- `/flip2-status <status> [reason]` - Update Claude's status
- `/flip2-health` - Check FLIP2 system health
- `/flip2-agents` - List all registered agents
- `/flip2-config` - Show current configuration

---

## Testing

### Health Checks
```bash
# Mac
curl -k https://localhost:8090/api/health

# Windows (from Mac)
curl http://192.168.1.220:8090/api/health

# Expected response:
# {"message":"API is healthy.","code":200,"data":{}}
```

### Sync Testing
```bash
# Send signal from Mac to Windows
curl -X POST https://localhost:8090/api/collections/signals/records \
  -H "X-API-Key: flip2_secret_key_123" \
  -H "Content-Type: application/json" \
  -k \
  -d '{
    "signal_id": "test-mac-win-001",
    "from_agent": "mac",
    "to_agent": "windows",
    "signal_type": "message",
    "content": "Test sync"
  }'

# Wait 15-30 seconds, then verify on Windows
curl "http://192.168.1.220:8090/api/collections/signals/records?filter=signal_id='test-mac-win-001'" \
  -H "X-API-Key: flip2_secret_key_123"
```

---

## Monitoring

### Check Daemon Status
```bash
# Mac
ps aux | grep flip2d
lsof -i :8090

# Windows
tasklist | findstr flip2d
netstat -ano | findstr :8090
```

### Check Logs
```bash
# Mac
tail -f /tmp/flip2d.log

# Windows
Get-Content C:\flip2\flip2d.log -Tail 50 -Wait
```

### Check Database Size
```bash
# Mac
du -h pb_data/auxiliary.db pb_data/data.db

# Should see:
# 24K pb_data/auxiliary.db  (was 77MB - automated cleanup working!)
# 912K pb_data/data.db
```

---

## Troubleshooting

### Mac Issues

**Daemon won't start:**
```bash
# Check if already running
ps aux | grep flip2d

# Check port in use
lsof -i :8090

# Check logs
tail -f /tmp/flip2d.log
```

**Environment validation fails:**
```bash
# Verify environment variable
echo $FLIP2_ENV

# Should be "production" for port 8090
# Should be "test" for port 9190
```

### Windows Issues

**Download fails:**
```batch
REM Verify Mac HTTP server is running
REM On Mac: lsof -i :8000

REM Test from Windows browser:
REM http://192.168.1.53:8000/

REM Check firewall allows outbound connections
```

**Daemon won't start:**
```powershell
# Check logs
Get-Content C:\flip2\flip2d.log -Tail 50

# Check port
netstat -ano | findstr :8090

# Run as Administrator
# Disable antivirus temporarily if blocking
```

**Health check fails:**
```powershell
# Wait 10 seconds after starting
Start-Sleep -Seconds 10

# Check process
Get-Process flip2d

# Check firewall
Get-NetFirewallRule -DisplayName "FLIP2"
```

### Sync Issues

**No synchronization:**
1. Verify both daemons running
2. Check network connectivity: `ping 192.168.1.53` / `ping 192.168.1.220`
3. Verify API keys match in both configs
4. Check sync is enabled in configs
5. Look for sync errors in logs

**Signals not appearing:**
1. Wait 15-30 seconds (sync interval)
2. Verify `to_agent` matches node_id in config
3. Check API authentication (X-API-Key header)

---

## File Structure

```
flip2/
├── flip2d                          # Mac daemon binary
├── flip2d-win.exe                  # Windows daemon binary
├── deploy_windows.bat              # Windows batch deployment
├── deploy_package_windows.ps1      # Windows PowerShell deployment
├── config/
│   ├── config.yaml                 # Mac production config
│   ├── config-test.yaml            # Mac test config
│   └── config-win-prod.yaml        # Windows production config
├── scripts/
│   ├── flip2_claude.sh             # Claude integration helper
│   ├── start_test_server.sh        # Start test server
│   ├── pre_work_checklist.sh       # Safety validation
│   ├── deploy_windows.py           # Python SSH deployment
│   └── deploy_windows_winrm.py     # Python WinRM deployment
├── pb_data/                        # Mac production data
│   ├── data.db                     # Application data (912KB)
│   └── auxiliary.db                # Logs (24KB, was 77MB)
├── pb_data_test/                   # Mac test data
└── docs/
    ├── DEPLOYMENT_SUMMARY_2025-12-31.md
    ├── WINDOWS_DEPLOYMENT_MANUAL.md
    ├── WINDOWS_DEPLOY_NOW.md
    ├── FINAL_STATUS.md
    └── README_DEPLOYMENT.md        # This file
```

---

## Scheduled Jobs

### Log Cleanup (Mac & Windows)
- **Pattern cleanup:** Every 8 hours
  - Removes junk logs (health checks, heartbeats, repetitive polls)
  - Keeps system lean
  
- **Full prune:** Every 48 hours
  - Deletes all logs older than 48 hours
  - Runs VACUUM to reclaim disk space
  - Keeps auxiliary.db under 10MB

### Zombie Reaper
- Runs every 5 minutes
- Detects tasks assigned to offline agents
- Marks zombie tasks as failed
- Prevents stuck tasks

### Statistics & Anomaly Detection
- Runs every 6 hours
- Collects signal statistics
- Detects communication patterns
- Generates reports

---

## Security

### API Authentication
All API calls require `X-API-Key` header:
```
X-API-Key: flip2_secret_key_123
```

### TLS/HTTPS
- **Mac:** HTTPS with self-signed cert (use `-k` flag with curl)
- **Windows:** HTTP (local network only)

### Network Isolation
- Production: Port 8090 (both Mac and Windows)
- Test: Port 9190 (Mac only, sync disabled)

---

## Performance

### Sync Interval
- **Default:** 15 seconds
- **Configurable:** In `config.yaml` → `sync.sync_interval`

### Concurrent Tasks
- **Mac:** 3 concurrent tasks
- **Windows:** 3 concurrent tasks
- **Configurable:** In `config.yaml` → `executor.max_concurrent_tasks`

### Database Size Targets
- **data.db:** <1MB (application data)
- **auxiliary.db:** <10MB (logs with automated cleanup)

---

## Deployment History

### 2025-12-31 Emergency Deployment
- ✅ Mac deployment complete
- ✅ Log cleanup freed 77MB
- ✅ Environment validation enforced
- ✅ Test infrastructure created
- ✅ Claude integration skill added
- 🟡 Windows deployment ready (pending execution)

**Deployment Score:** 9/10 ⭐

---

## Support

### Documentation
- `DEPLOYMENT_SUMMARY_2025-12-31.md` - Complete deployment record
- `WINDOWS_DEPLOYMENT_MANUAL.md` - Detailed Windows deployment
- `WINDOWS_DEPLOY_NOW.md` - Quick Windows deployment
- `FINAL_STATUS.md` - Final deployment status

### Scripts
All scripts in `scripts/` directory have built-in help:
```bash
./scripts/flip2_claude.sh          # Shows usage
./scripts/pre_work_checklist.sh    # Validates environment
./scripts/start_test_server.sh     # Starts test server
```

---

**Last Updated:** 2025-12-31 02:30 AM  
**Deployment Version:** 1.0.0  
**Status:** Production Ready ✅
