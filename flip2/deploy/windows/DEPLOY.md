# Windows FLIP2 Deployment

## Quick Start

1. **Copy files to Windows**:
   ```powershell
   # Create directory
   mkdir C:\flip2

   # Copy files (from Mac via network share or SCP)
   # flip2d.exe, flip2.exe, config.yaml -> C:\flip2\
   ```

2. **Initialize PocketBase data**:
   ```powershell
   cd C:\flip2

   # First run will bootstrap collections
   .\flip2d.exe --config config.yaml --foreground

   # Wait for "FLIP2 DAEMON STARTING" message
   # Press Ctrl+C after initialization
   ```

3. **Start as background service**:
   ```powershell
   # Option A: Simple background
   Start-Process -NoNewWindow .\flip2d.exe -ArgumentList "--config","config.yaml","--foreground"

   # Option B: As Windows service (recommended)
   # Use NSSM or similar to install as service
   ```

4. **Verify**:
   ```powershell
   # Check health
   curl http://localhost:8090/api/health

   # Check signals
   .\flip2 signal list
   ```

## Files Included

- `flip2d.exe` - Daemon binary
- `flip2.exe` - CLI tool
- `config.yaml` - Configuration (ports, sync peers, etc.)

## Configuration Notes

- Production port: 8090
- Sync peer (Mac): http://192.168.1.53:8090
- Data directory: C:\flip2\pb_data (created on first run)
- Log file: C:\flip2\flip2d.log

## Troubleshooting

**Daemon won't start**:
```powershell
# Check if port in use
netstat -an | findstr 8090

# Run in foreground to see errors
.\flip2d.exe --config config.yaml --foreground
```

**Sync not working**:
```powershell
# Check Mac is reachable
ping 192.168.1.53

# Check Mac daemon responding
curl http://192.168.1.53:8090/api/health
```

**Environment validation error**:
- Ensure FLIP2_ENV is not set, or set to "production"
- Ensure port is 8090 (production range 8xxx)
