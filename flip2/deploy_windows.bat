@echo off
REM FLIP2 Windows Deployment Batch Script
REM Download this file and run on Windows PC

echo === FLIP2 Windows Deployment ===
echo.

REM Configuration
set FLIP2_DIR=C:\flip2
set MAC_IP=192.168.1.53
set MAC_PORT=8000

REM Step 1: Create directories
echo 1. Creating directory structure...
mkdir %FLIP2_DIR% 2>nul
mkdir %FLIP2_DIR%\pb_data 2>nul
mkdir %FLIP2_DIR%\archives 2>nul
mkdir %FLIP2_DIR%\archives\signals 2>nul
echo   Directories created

REM Step 2: Stop old daemon
echo.
echo 2. Stopping old daemon...
taskkill /F /IM flip2d.exe 2>nul
timeout /t 2 /nobreak >nul

REM Step 3: Download files
echo.
echo 3. Downloading files from Mac...
echo   Downloading flip2d-win.exe (35MB)...
powershell -Command "& {$ProgressPreference = 'SilentlyContinue'; Invoke-WebRequest -Uri 'http://%MAC_IP%:%MAC_PORT%/flip2d-win.exe' -OutFile '%FLIP2_DIR%\flip2d-new.exe' -UseBasicParsing}"

echo   Downloading config.yaml...
powershell -Command "& {$ProgressPreference = 'SilentlyContinue'; Invoke-WebRequest -Uri 'http://%MAC_IP%:%MAC_PORT%/config-win-prod.yaml' -OutFile '%FLIP2_DIR%\config-new.yaml' -UseBasicParsing}"

if not exist %FLIP2_DIR%\flip2d-new.exe (
    echo   ERROR: Download failed!
    echo   Make sure Mac HTTP server is running on %MAC_IP%:%MAC_PORT%
    pause
    exit /b 1
)

echo   Download complete

REM Step 4: Backup and install
echo.
echo 4. Installing files...
if exist %FLIP2_DIR%\flip2d.exe (
    copy %FLIP2_DIR%\flip2d.exe %FLIP2_DIR%\flip2d-backup.exe >nul 2>&1
)
move /Y %FLIP2_DIR%\flip2d-new.exe %FLIP2_DIR%\flip2d.exe >nul
move /Y %FLIP2_DIR%\config-new.yaml %FLIP2_DIR%\config.yaml >nul
echo   Files installed

REM Step 5: Configure firewall
echo.
echo 5. Configuring firewall...
powershell -Command "& {$rule = Get-NetFirewallRule -DisplayName 'FLIP2' -ErrorAction SilentlyContinue; if (!$rule) {New-NetFirewallRule -DisplayName 'FLIP2' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8090 | Out-Null; Write-Output 'Firewall rule added'} else {Write-Output 'Firewall rule exists'}}"

REM Step 6: Start daemon
echo.
echo 6. Starting daemon...
cd /d %FLIP2_DIR%
set FLIP2_ENV=production
start /B flip2d.exe --config config.yaml

echo   Daemon started
timeout /t 5 /nobreak >nul

REM Step 7: Verify
echo.
echo 7. Verifying deployment...
powershell -Command "& {try {$response = Invoke-WebRequest -Uri 'http://localhost:8090/api/health' -UseBasicParsing -TimeoutSec 10; if ($response.StatusCode -eq 200) {Write-Output 'Health check: PASSED'; Write-Output $response.Content} else {Write-Output 'Health check: FAILED'}} catch {Write-Output 'Health check: FAILED'; Write-Output $_.Exception.Message}}"

echo.
echo === Deployment Complete ===
echo   API: http://localhost:8090
echo   Logs: %FLIP2_DIR%\flip2d.log
echo.
echo Test from Mac: curl http://192.168.1.220:8090/api/health
echo.
pause
