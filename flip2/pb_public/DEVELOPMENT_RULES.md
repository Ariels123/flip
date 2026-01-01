# FLIP2 Development Rules & Best Practices

**Effective Date:** 2025-12-18
**Agreed By:** claude-mac, claude-win (pending confirmation)

---

## Rule 1: Test Server Requirement

**ANY modification that affects the FLIP2 server structurally MUST be tested on a test server running on a separate port BEFORE deployment to production.**

### Production Ports (DO NOT MODIFY WITHOUT TESTING)
| Service | Mac Port | Windows Port |
|---------|----------|--------------|
| PocketBase API | 8091 | 8090 |
| Knowledge Base | 8092 | - |

### Test Server Ports
| Service | Mac Port | Windows Port |
|---------|----------|--------------|
| Test PocketBase | 9191 | 9190 |
| Test KB | 9192 | - |

### What Requires Test Server First
- Database schema changes (migrations)
- Collection modifications (fields, rules, indexes)
- API endpoint changes
- TLS/security configuration changes
- Sync protocol modifications
- Any daemon startup/shutdown logic changes

### What Can Be Done Directly
- Adding new standalone features (new files, new packages)
- Bug fixes that don't change data structures
- Documentation updates
- Config value changes (non-structural)

---

## Rule 2: Single Repository

**All FLIP2 code MUST exist in ONE authoritative repository to prevent version drift and confusion.**

### Repository Structure
```
flip2/
├── cmd/flip2d/          # Daemon entry point
├── internal/            # All internal packages
├── pb_data/             # PocketBase data (gitignored)
├── pb_migrations/       # Database migrations
├── config/              # Configuration files
├── certs/               # TLS certificates (gitignored)
├── archives/            # Signal archives
└── DEVELOPMENT_RULES.md # This file
```

### Sync Process
1. Mac is the PRIMARY development machine
2. Changes are tested locally on Mac first
3. Tested changes are synced to Windows via:
   - Git (preferred) OR
   - SFTP transfer with verification
4. Windows validates changes work on Windows platform

---

## Rule 3: Stable Version Protection

**A stable, operational version of FLIP2 MUST remain running at all times.**

### Deployment Process
1. **Never** stop the production daemon for untested changes
2. Build new version with different binary name (e.g., `flip2d-v2.exe`)
3. Test new version on test port
4. Only after ALL tests pass:
   - Stop old daemon
   - Rename new binary to `flip2d`
   - Start new daemon
   - Verify production functionality
5. Keep previous stable binary as backup (`flip2d.backup`)

### Rollback Procedure
If new version fails in production:
```bash
# Immediate rollback
taskkill /F /IM flip2d.exe  # Windows
pkill flip2d                 # Mac

# Restore backup
mv flip2d.backup flip2d
./flip2d &
```

---

## Rule 4: Communication Lines Must Stay Open

**The signals collection and PocketBase API must remain operational during any development work.**

### Before Any Structural Change
1. Verify current communication works:
   ```bash
   curl http://localhost:8090/api/collections/signals/records
   ```
2. Document current state
3. Make changes on TEST server first
4. Verify test server works
5. Only then apply to production

### Emergency Communication Channels (if PB fails)
1. SSH direct access
2. Knowledge Base (KB) file sharing
3. /tmp signal files

---

## Rule 5: Testing Requirements

### Before Merging/Deploying Any Change
1. All existing tests must pass
2. New tests for new functionality
3. Manual verification of:
   - API endpoints work
   - Signals can be sent/received
   - Sync between Mac/Windows works

### Test Commands
```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/archiver/...

# Run with verbose output
go test -v ./...
```

---

## Acknowledgment

Both agents must confirm agreement to these rules by sending a signal:

```json
{
  "signal_type": "agreement",
  "content": "CONFIRMED: Development Rules v1.0 accepted"
}
```

---

## Version History
- v1.0 (2025-12-18): Initial rules established
