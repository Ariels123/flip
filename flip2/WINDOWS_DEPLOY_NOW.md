# DEPLOY TO WINDOWS NOW - Quick Instructions

## Method 1: HTTP Download (Recommended)

### On Mac (Already Done):
HTTP server running on port 8000 ✓
Files ready: flip2d-win.exe, config-win-prod.yaml ✓

### On Windows PC (192.168.1.220):

1. Open PowerShell as Administrator

2. Download and run deployment script:
```powershell
Invoke-WebRequest -Uri "http://192.168.1.53:8000/deploy_package_windows.ps1" -OutFile "$env:TEMP\deploy_flip2.ps1"
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
& "$env:TEMP\deploy_flip2.ps1"
```

The script will:
- Create C:\flip2\ directory structure
- Download flip2d-win.exe and config.yaml from Mac
- Stop old daemon
- Install new files
- Configure firewall
- Start daemon
- Verify health

---

## Method 2: Manual Copy (If HTTP fails)

### Copy files to USB/Network share:
1. On Mac: Copy these files to USB
   - flip2d-win.exe
   - config-win-prod.yaml

2. On Windows: Copy from USB to C:\flip2\

3. On Windows PowerShell (as Administrator):
```powershell
# Create directories
mkdir C:\flip2\pb_data -Force
mkdir C:\flip2\archives\signals -Force

# Stop old daemon
Stop-Process -Name flip2d -Force -ErrorAction SilentlyContinue

# Install files (if copied to USB as D:\)
Copy-Item D:\flip2d-win.exe C:\flip2\flip2d.exe -Force
Copy-Item D:\config-win-prod.yaml C:\flip2\config.yaml -Force

# Configure firewall
New-NetFirewallRule -DisplayName "FLIP2" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8090

# Start daemon
cd C:\flip2
$env:FLIP2_ENV = "production"
Start-Process flip2d.exe -ArgumentList "--config config.yaml" -NoNewWindow

# Wait and check
Start-Sleep -Seconds 5
Invoke-WebRequest http://localhost:8090/api/health
```

---

## Verification (From Mac):

```bash
# Test Windows health endpoint
curl http://192.168.1.220:8090/api/health

# Should return:
# {"message":"API is healthy.","code":200,"data":{}}
```

---

## Files Available on Mac HTTP Server:

http://192.168.1.53:8000/flip2d-win.exe (35.1 MB)
http://192.168.1.53:8000/config-win-prod.yaml (1.1 KB)
http://192.168.1.53:8000/deploy_package_windows.ps1 (Deployment script)

---

## Quick One-Liner (Windows PowerShell):

```powershell
iwr "http://192.168.1.53:8000/deploy_package_windows.ps1" -OutFile "$env:TEMP\d.ps1"; Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass; & "$env:TEMP\d.ps1"
```

---

## Troubleshooting:

**Can't download from Mac:**
- Verify HTTP server running: `lsof -i :8000` (on Mac)
- Check firewall allows port 8000
- Try from Windows browser: http://192.168.1.53:8000/

**Daemon won't start:**
- Check logs: `Get-Content C:\flip2\flip2d.log -Tail 50`
- Verify port 8090 not in use: `netstat -ano | findstr :8090`
- Run as Administrator

**Health check fails:**
- Wait 10 seconds after starting
- Check process: `Get-Process flip2d`
- Check firewall: Windows Defender might be blocking

---

## What Happens After Deployment:

1. ✅ Windows daemon runs on port 8090
2. ✅ Syncs with Mac every 15 seconds
3. ✅ Auto-cleanup logs (48h retention)
4. ✅ Bootstrap creates collections automatically
5. ✅ Ready for Mac ↔ Windows sync testing

---

**Status:** Files ready ✓ | HTTP server ready ✓ | Waiting for Windows deployment
