# FLIP2 Windows Deployment Script
# Run this directly on Windows PC

$ErrorActionPreference = "Stop"

Write-Host "=== FLIP2 Windows Deployment ===" -ForegroundColor Cyan
Write-Host ""

# Configuration
$FLIP2_DIR = "C:\flip2"
$BINARY_URL = "http://192.168.1.53:8000/flip2d-win.exe"
$CONFIG_URL = "http://192.168.1.53:8000/config-win-prod.yaml"

# Step 1: Create directories
Write-Host "1. Creating directory structure..." -ForegroundColor Yellow
$dirs = @(
    "$FLIP2_DIR",
    "$FLIP2_DIR\pb_data",
    "$FLIP2_DIR\archives",
    "$FLIP2_DIR\archives\signals"
)

foreach ($dir in $dirs) {
    if (!(Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir | Out-Null
        Write-Host "  Created: $dir" -ForegroundColor Green
    } else {
        Write-Host "  Exists: $dir" -ForegroundColor Gray
    }
}

# Step 2: Stop old daemon
Write-Host "`n2. Stopping existing daemon..." -ForegroundColor Yellow
$process = Get-Process -Name "flip2d" -ErrorAction SilentlyContinue
if ($process) {
    Stop-Process -Name "flip2d" -Force
    Write-Host "  Stopped old daemon" -ForegroundColor Green
    Start-Sleep -Seconds 2
} else {
    Write-Host "  No daemon running" -ForegroundColor Gray
}

# Step 3: Download files
Write-Host "`n3. Downloading files from Mac..." -ForegroundColor Yellow
Write-Host "  Note: Make sure Mac HTTP server is running!" -ForegroundColor Cyan
Write-Host "  On Mac run: python3 -m http.server 8000" -ForegroundColor Cyan
Write-Host ""

try {
    # Download binary
    Write-Host "  Downloading flip2d-win.exe..."
    Invoke-WebRequest -Uri $BINARY_URL -OutFile "$FLIP2_DIR\flip2d-new.exe" -UseBasicParsing
    Write-Host "  Downloaded binary ($(( (Get-Item "$FLIP2_DIR\flip2d-new.exe").Length / 1MB ).ToString('F1')) MB)" -ForegroundColor Green
    
    # Download config
    Write-Host "  Downloading config.yaml..."
    Invoke-WebRequest -Uri $CONFIG_URL -OutFile "$FLIP2_DIR\config-new.yaml" -UseBasicParsing
    Write-Host "  Downloaded config" -ForegroundColor Green
} catch {
    Write-Host "  ERROR: Download failed!" -ForegroundColor Red
    Write-Host "  Make sure Mac HTTP server is running:" -ForegroundColor Yellow
    Write-Host "    cd /Users/arielspivakovsky/src/flip/flip2" -ForegroundColor White
    Write-Host "    python3 -m http.server 8000" -ForegroundColor White
    Write-Host ""
    Write-Host "  Alternative: Copy files manually to C:\flip2\" -ForegroundColor Yellow
    exit 1
}

# Step 4: Backup and install
Write-Host "`n4. Installing new binary..." -ForegroundColor Yellow
if (Test-Path "$FLIP2_DIR\flip2d.exe") {
    Copy-Item "$FLIP2_DIR\flip2d.exe" "$FLIP2_DIR\flip2d-backup.exe" -Force
    Write-Host "  Backed up old binary" -ForegroundColor Gray
}

Move-Item "$FLIP2_DIR\flip2d-new.exe" "$FLIP2_DIR\flip2d.exe" -Force
Move-Item "$FLIP2_DIR\config-new.yaml" "$FLIP2_DIR\config.yaml" -Force
Write-Host "  Installed new files" -ForegroundColor Green

# Step 5: Configure firewall
Write-Host "`n5. Configuring firewall..." -ForegroundColor Yellow
$firewallRule = Get-NetFirewallRule -DisplayName "FLIP2" -ErrorAction SilentlyContinue
if (!$firewallRule) {
    New-NetFirewallRule -DisplayName "FLIP2" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8090 | Out-Null
    Write-Host "  Firewall rule added" -ForegroundColor Green
} else {
    Write-Host "  Firewall rule exists" -ForegroundColor Gray
}

# Step 6: Start daemon
Write-Host "`n6. Starting daemon..." -ForegroundColor Yellow
Set-Location $FLIP2_DIR
$env:FLIP2_ENV = "production"

$processInfo = New-Object System.Diagnostics.ProcessStartInfo
$processInfo.FileName = "$FLIP2_DIR\flip2d.exe"
$processInfo.Arguments = "--config config.yaml"
$processInfo.WorkingDirectory = $FLIP2_DIR
$processInfo.UseShellExecute = $false
$processInfo.CreateNoWindow = $true
[System.Diagnostics.Process]::Start($processInfo) | Out-Null

Write-Host "  Daemon started" -ForegroundColor Green
Start-Sleep -Seconds 5

# Step 7: Verify
Write-Host "`n7. Verifying deployment..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8090/api/health" -UseBasicParsing -TimeoutSec 10
    $health = $response.Content | ConvertFrom-Json
    
    if ($health.code -eq 200) {
        Write-Host "  Health check: PASSED" -ForegroundColor Green
        Write-Host ""
        Write-Host "=== Deployment Successful ===" -ForegroundColor Green
        Write-Host "  API: http://localhost:8090" -ForegroundColor White
        Write-Host "  Logs: C:\flip2\flip2d.log" -ForegroundColor White
        Write-Host ""
        Write-Host "Test from Mac:" -ForegroundColor Cyan
        Write-Host "  curl http://192.168.1.220:8090/api/health" -ForegroundColor White
    } else {
        throw "Unexpected response code"
    }
} catch {
    Write-Host "  Health check: FAILED" -ForegroundColor Red
    Write-Host ""
    Write-Host "Check logs for errors:" -ForegroundColor Yellow
    Write-Host "  Get-Content C:\flip2\flip2d.log -Tail 50" -ForegroundColor White
}
