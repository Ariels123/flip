#!/usr/bin/env python3
"""
Windows Deployment using WinRM (Windows Remote Management)
Alternative to SSH for Windows deployment
"""

import sys
import winrm
import time
import os

# Configuration
WINDOWS_HOST = "192.168.1.220"
WINDOWS_USER = "Agnizar"
WINDOWS_PASSWORD = os.getenv("WINDOWS_PASSWORD", "")
MAC_IP = "192.168.1.53"

def deploy_via_winrm():
    """Deploy FLIP2 to Windows using WinRM"""
    
    print("=== FLIP2 Windows Deployment via WinRM ===\n")
    
    if not WINDOWS_PASSWORD:
        print("Error: WINDOWS_PASSWORD environment variable not set")
        print("Usage: export WINDOWS_PASSWORD='your_password'")
        print("       python3 scripts/deploy_windows_winrm.py")
        return False
    
    print(f"Connecting to {WINDOWS_HOST} via WinRM...")
    
    try:
        # Create WinRM session
        session = winrm.Session(
            f'http://{WINDOWS_HOST}:5985/wsman',
            auth=(WINDOWS_USER, WINDOWS_PASSWORD),
            transport='ntlm'
        )
        
        # Test connection
        result = session.run_cmd('echo', ['Connection test'])
        if result.status_code != 0:
            print(f"Connection test failed: {result.std_err.decode()}")
            return False
        
        print("✓ Connected to Windows via WinRM\n")
        
        # Step 1: Create directories
        print("1. Creating directory structure...")
        commands = [
            'mkdir C:\\flip2 2>nul || echo Directory exists',
            'mkdir C:\\flip2\\pb_data 2>nul || echo pb_data exists',
            'mkdir C:\\flip2\\archives 2>nul || echo archives exists',
            'mkdir C:\\flip2\\archives\\signals 2>nul || echo signals exists'
        ]
        
        for cmd in commands:
            result = session.run_cmd('cmd', ['/c', cmd])
            if result.std_out:
                print(f"  {result.std_out.decode().strip()}")
        
        # Step 2: Stop old daemon
        print("\n2. Stopping old daemon...")
        result = session.run_cmd('taskkill', ['/F', '/IM', 'flip2d.exe'])
        if "SUCCESS" in result.std_out.decode():
            print("  Stopped old daemon")
        else:
            print("  No daemon running")
        
        time.sleep(2)
        
        # Step 3: Download files from Mac HTTP server
        print("\n3. Downloading files from Mac...")
        
        download_script = f'''
$ProgressPreference = 'SilentlyContinue'
Invoke-WebRequest -Uri "http://{MAC_IP}:8000/flip2d-win.exe" -OutFile "C:\\flip2\\flip2d-new.exe" -UseBasicParsing
Invoke-WebRequest -Uri "http://{MAC_IP}:8000/config-win-prod.yaml" -OutFile "C:\\flip2\\config-new.yaml" -UseBasicParsing
'''
        
        result = session.run_ps(download_script)
        if result.status_code == 0:
            print("  ✓ Downloaded files")
        else:
            print(f"  ✗ Download failed: {result.std_err.decode()}")
            return False
        
        # Step 4: Install files
        print("\n4. Installing files...")
        install_commands = [
            'copy C:\\flip2\\flip2d.exe C:\\flip2\\flip2d-backup.exe 2>nul || echo No old binary',
            'move /Y C:\\flip2\\flip2d-new.exe C:\\flip2\\flip2d.exe',
            'move /Y C:\\flip2\\config-new.yaml C:\\flip2\\config.yaml'
        ]
        
        for cmd in install_commands:
            result = session.run_cmd('cmd', ['/c', cmd])
        
        print("  ✓ Files installed")
        
        # Step 5: Configure firewall
        print("\n5. Configuring firewall...")
        firewall_script = '''
$rule = Get-NetFirewallRule -DisplayName "FLIP2" -ErrorAction SilentlyContinue
if (!$rule) {
    New-NetFirewallRule -DisplayName "FLIP2" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8090 | Out-Null
    Write-Output "Firewall rule added"
} else {
    Write-Output "Firewall rule exists"
}
'''
        result = session.run_ps(firewall_script)
        print(f"  {result.std_out.decode().strip()}")
        
        # Step 6: Start daemon
        print("\n6. Starting daemon...")
        start_script = '''
cd C:\\flip2
$env:FLIP2_ENV = "production"
$process = Start-Process -FilePath "C:\\flip2\\flip2d.exe" -ArgumentList "--config config.yaml" -PassThru -NoNewWindow
Write-Output "Daemon started (PID: $($process.Id))"
'''
        result = session.run_ps(start_script)
        print(f"  {result.std_out.decode().strip()}")
        
        print("\n7. Waiting for daemon to start...")
        time.sleep(5)
        
        # Step 8: Verify health
        print("\n8. Verifying deployment...")
        health_script = '''
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8090/api/health" -UseBasicParsing -TimeoutSec 10
    $response.Content
} catch {
    Write-Error $_.Exception.Message
}
'''
        result = session.run_ps(health_script)
        
        if '"message":"API is healthy"' in result.std_out.decode():
            print("  ✓ Health check PASSED")
            print("\n=== Deployment Successful ===")
            print(f"  API: http://{WINDOWS_HOST}:8090")
            print(f"  Health: http://{WINDOWS_HOST}:8090/api/health")
            return True
        else:
            print("  ✗ Health check FAILED")
            print(f"  Output: {result.std_out.decode()}")
            print(f"  Error: {result.std_err.decode()}")
            return False
        
    except Exception as e:
        print(f"\n✗ Deployment failed: {e}")
        import traceback
        traceback.print_exc()
        return False

if __name__ == "__main__":
    success = deploy_via_winrm()
    sys.exit(0 if success else 1)
