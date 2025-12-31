# FLIP - File-based LLM Inter-Process Communication

**Version:** 5.2 → FLIP2 1.0  
**Status:** Production  
**Repository:** https://github.com/Ariels123/flip

---

## Overview

FLIP is a multi-agent coordination system that enables Claude, Gemini, and other LLM agents to work together on complex tasks through structured communication, task management, and distributed synchronization.

**FLIP2** is a complete rewrite with:
- PocketBase backend (REST API + real-time subscriptions)
- Proper daemon architecture
- MySQL support for production
- Vector clock-based synchronization
- Automated log management
- Environment validation for safety

---

## Quick Start

### Mac Installation
```bash
cd flip2
FLIP2_ENV=production ./flip2d --config config/config.yaml
```

### Windows Installation
```batch
REM Download and run:
iwr "http://192.168.1.53:8000/deploy_windows.bat" -OutFile "$env:TEMP\deploy.bat"
cmd /c "$env:TEMP\deploy.bat"
```

### Verification
```bash
# Mac
curl -k https://localhost:8090/api/health

# Windows
curl http://192.168.1.220:8090/api/health
```

---

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│                     FLIP2 SYSTEM                           │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌─────────────┐                        ┌─────────────┐   │
│  │ Mac Node    │   ← Sync (15s) →      │ Win Node    │   │
│  │ flip2d:8090 │                        │ flip2d:8090 │   │
│  │ (SQLite)    │                        │ (SQLite)    │   │
│  └─────────────┘                        └─────────────┘   │
│         │                                       │          │
│         └───────────────┬───────────────────────┘          │
│                         │                                  │
│                 ┌───────▼────────┐                         │
│                 │   PocketBase   │                         │
│                 │  • REST API    │                         │
│                 │  • Realtime    │                         │
│                 │  • Bootstrap   │                         │
│                 └────────────────┘                         │
│                                                            │
│  ┌─────────────────────────────────────────────────────┐  │
│  │                    AGENTS                            │  │
│  │  Claude • Gemini • Antigravity • Custom Plugins     │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

---

## Key Features

### ✅ Multi-Agent Coordination
- **Task Assignment** - Assign tasks to specific agents
- **Signal System** - Inter-agent communication with priorities
- **Handoff Protocol** - Formal task ownership transfers
- **Status Tracking** - Real-time agent status (online/offline/busy/idle)

### ✅ Distributed Sync
- **Vector Clocks** - CRDT-based conflict resolution
- **Bidirectional** - Mac ↔ Windows synchronization
- **Resilient** - Handles offline/online scenarios
- **Configurable** - 15-second sync interval (adjustable)

### ✅ Safety & Reliability
- **Environment Validation** - Prevents prod/test confusion
- **Automated Log Cleanup** - 48h retention + VACUUM
- **Test Infrastructure** - Safe testing on port 9190
- **Pre-work Checklist** - Validates before deployment
- **Zombie Reaper** - Detects and handles stale tasks

### ✅ Claude Integration
- **FLIP2 Skill** - `/flip2-*` commands for Claude
- **Helper Scripts** - `flip2_claude.sh` for CLI usage
- **Agent Registration** - Claude as first-class FLIP agent
- **Inbox/Outbox** - Check signals and send messages

---

## Project Structure

```
flip/
├── README.md                       # This file
├── CLAUDE.md                       # Instructions for Claude agents
├── flip2/                          # FLIP2 implementation
│   ├── README_DEPLOYMENT.md        # Deployment guide
│   ├── DEPLOYMENT_SUMMARY.md       # Latest deployment status
│   ├── FINAL_STATUS.md             # Current system status
│   ├── cmd/
│   │   ├── flip2/                  # CLI tool
│   │   └── flip2d/                 # Daemon
│   ├── internal/                   # Core implementation
│   │   ├── daemon/                 # Daemon logic
│   │   ├── sync/                   # Sync engine
│   │   ├── archiver/               # Message archiving
│   │   ├── executor/               # Task execution
│   │   └── scheduler/              # Job scheduling
│   ├── config/
│   │   ├── config.yaml             # Mac production
│   │   ├── config-test.yaml        # Mac test
│   │   └── config-win-prod.yaml    # Windows production
│   ├── scripts/
│   │   ├── flip2_claude.sh         # Claude integration
│   │   ├── pre_work_checklist.sh   # Safety validation
│   │   └── deploy_windows.py       # Windows deployment
│   └── docs/                       # Additional documentation
├── FlipDevLog/                     # Development history
│   └── dev_log.md                  # Version history (v1-v5.2)
└── docs/
    └── roadmap.md                  # Feature roadmap
```

---

## Version History

### FLIP v1-v5 (Legacy)
- SQLite-based file coordination
- Basic agent spawning
- WebSocket server
- Multi-agent pipelines

### FLIP2 v1.0 (Current) ⭐
**Released:** 2025-12-31

**Major Changes:**
- ✅ PocketBase backend (REST API + real-time)
- ✅ Proper daemon architecture (systemd/launchd)
- ✅ Environment validation (prod/test safety)
- ✅ Automated log management (freed 77MB immediately)
- ✅ Vector clock synchronization
- ✅ Bootstrap collection creation
- ✅ Test infrastructure (port 9190)
- ✅ Claude skill integration

**Deployment Status:**
- Mac: ✅ COMPLETE (PID 74066, port 8090)
- Windows: 🟡 READY (awaiting deployment)

**Metrics:**
- Disk reclaimed: 77MB → 24KB (auxiliary.db)
- Log volume: 141K → 0 (auto-cleanup active)
- Deployment score: 9/10

---

## Configuration

### Environment Variables
```bash
# Deployment environment
export FLIP2_ENV=production    # or "test"

# FLIP2 Claude Integration
export FLIP2_URL=https://localhost:8090
export FLIP2_API_KEY=flip2_secret_key_123
export FLIP2_AGENT_ID=claude-mac
```

### Port Convention
- **Production:** 8090 (both Mac and Windows)
- **Test:** 9190 (Mac only, sync disabled)

### API Authentication
All API calls require:
```
X-API-Key: flip2_secret_key_123
```

---

## Usage

### Start Daemon
```bash
# Mac Production
FLIP2_ENV=production ./flip2d --config config/config.yaml

# Mac Test
FLIP2_ENV=test ./scripts/start_test_server.sh

# Windows
cd C:\flip2
set FLIP2_ENV=production
flip2d.exe --config config.yaml
```

### Claude Integration
```bash
# Register Claude as agent
./scripts/flip2_claude.sh register claude-coordinator

# Check inbox
./scripts/flip2_claude.sh inbox

# Send signal
./scripts/flip2_claude.sh send gemini-worker "Analyze logs"

# List all agents
./scripts/flip2_claude.sh agents

# Check health
./scripts/flip2_claude.sh health
```

### Health Check
```bash
# Mac
curl -k https://localhost:8090/api/health

# Windows
curl http://192.168.1.220:8090/api/health

# Expected: {"message":"API is healthy.","code":200,"data":{}}
```

---

## Development

### Building
```bash
# Mac
go build -o flip2d ./cmd/flip2d

# Windows (cross-compile from Mac)
GOOS=windows GOARCH=amd64 go build -o flip2d-win.exe ./cmd/flip2d
```

### Testing
```bash
# Start test server
./scripts/start_test_server.sh

# Run pre-work checklist
./scripts/pre_work_checklist.sh

# Test health
curl http://localhost:9190/api/health
```

### Safety Validation
```bash
# Before any FLIP2 work
./scripts/pre_work_checklist.sh

# Validates:
# - FLIP2_ENV matches port
# - No port conflicts
# - Database sizes OK
# - ABORTS if mismatch detected
```

---

## Monitoring

### Check Status
```bash
# Daemon running?
ps aux | grep flip2d              # Mac
tasklist | findstr flip2d         # Windows

# Port in use?
lsof -i :8090                     # Mac
netstat -ano | findstr :8090      # Windows

# Check logs
tail -f /tmp/flip2d.log                        # Mac
Get-Content C:\flip2\flip2d.log -Tail 50       # Windows
```

### Database Sizes
```bash
# Mac
du -h pb_data/auxiliary.db pb_data/data.db

# Target sizes:
# auxiliary.db: <10MB (automated cleanup)
# data.db: <1MB (application data)
```

---

## Deployment

### Mac → Windows Sync Setup
1. Mac running on `192.168.1.53:8090`
2. Windows running on `192.168.1.220:8090`
3. Sync every 15 seconds
4. Vector clocks handle conflicts

### Windows Deployment Methods

**Method 1: Batch Script (Easiest)**
```batch
iwr "http://192.168.1.53:8000/deploy_windows.bat" -OutFile "$env:TEMP\d.bat"
cmd /c "$env:TEMP\d.bat"
```

**Method 2: PowerShell Script**
```powershell
iwr "http://192.168.1.53:8000/deploy_package_windows.ps1" -OutFile "$env:TEMP\d.ps1"
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
& "$env:TEMP\d.ps1"
```

**Method 3: Manual Copy**
See `flip2/WINDOWS_DEPLOYMENT_MANUAL.md`

---

## Troubleshooting

### Common Issues

**Environment validation fails:**
```bash
# Error: "Production environment cannot use test port"
# Fix: Set correct environment variable
export FLIP2_ENV=production  # for port 8090
export FLIP2_ENV=test        # for port 9190
```

**Disk space issues:**
```bash
# Check auxiliary.db size
du -h pb_data/auxiliary.db

# If >50MB, manually cleanup:
sqlite3 pb_data/auxiliary.db "DELETE FROM _logs WHERE created < datetime('now', '-48 hours'); VACUUM;"
```

**Sync not working:**
1. Verify both daemons running
2. Check network: `ping 192.168.1.53` / `ping 192.168.1.220`
3. Verify API keys match in configs
4. Check logs for sync errors

**Health check fails:**
1. Wait 10 seconds after starting
2. Check process running
3. Check port not blocked by firewall
4. Verify config has correct port

---

## Documentation

### Main Docs
- `README.md` - This file (overview)
- `CLAUDE.md` - Instructions for Claude agents
- `flip2/README_DEPLOYMENT.md` - Deployment guide
- `flip2/DEPLOYMENT_SUMMARY.md` - Latest deployment
- `flip2/FINAL_STATUS.md` - Current status

### Development Docs
- `FlipDevLog/dev_log.md` - Version history
- `docs/roadmap.md` - Feature roadmap
- `flip2/ProjectDocs/` - Architecture docs

### Deployment Docs
- `flip2/WINDOWS_DEPLOYMENT_MANUAL.md` - Windows step-by-step
- `flip2/WINDOWS_DEPLOY_NOW.md` - Quick Windows deploy
- `flip2/COMPREHENSIVE_REVIEW_2025-12-20.md` - Security review

---

## Contributing

### Workflow
1. Make changes in test environment (port 9190)
2. Run safety checklist: `./scripts/pre_work_checklist.sh`
3. Test thoroughly
4. Deploy to production (port 8090)
5. Monitor logs and health

### Coding Standards
- Go code in `internal/`
- Scripts in `scripts/`
- Configs in `config/`
- Documentation in `docs/` or `flip2/`

---

## License

MIT License - See LICENSE file

---

## Contact

- **Repository:** https://github.com/Ariels123/flip
- **Issues:** https://github.com/Ariels123/flip/issues

---

**Last Updated:** 2025-12-31  
**Version:** FLIP2 v1.0  
**Status:** ✅ Production Ready  
**Deployment:** Mac ✅ | Windows 🟡
