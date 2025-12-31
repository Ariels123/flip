#!/usr/bin/env python3
"""
Windows Deployment Script using Paramiko
Workaround for SSH authentication issues
"""

import paramiko
import sys
import time
import os
from pathlib import Path

# Windows connection details
WINDOWS_HOST = "192.168.1.220"
WINDOWS_USER = "Agnizar"
WINDOWS_PORT = 22

def create_ssh_client():
    """Create SSH client with auto-add policy for host keys"""
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    return client

def execute_command(ssh, command, timeout=30):
    """Execute command on Windows via SSH"""
    print(f"  Executing: {command[:80]}...")
    stdin, stdout, stderr = ssh.exec_command(command, timeout=timeout)
    
    output = stdout.read().decode('utf-8', errors='ignore')
    error = stderr.read().decode('utf-8', errors='ignore')
    exit_code = stdout.channel.recv_exit_status()
    
    if output:
        print(f"    Output: {output[:200]}")
    if error and exit_code != 0:
        print(f"    Error: {error[:200]}")
    
    return exit_code, output, error

def deploy_to_windows():
    """Deploy FLIP2 to Windows"""
    print("=== Windows Deployment via Python/Paramiko ===\n")
    
    # Paths
    base_dir = Path(__file__).parent.parent
    binary_path = base_dir / "flip2d-win.exe"
    config_path = base_dir / "config-win-prod.yaml"
    
    # Verify local files exist
    if not binary_path.exists():
        print(f"❌ Binary not found: {binary_path}")
        return False
    
    if not config_path.exists():
        print(f"❌ Config not found: {config_path}")
        return False
    
    print(f"✓ Binary found: {binary_path} ({binary_path.stat().st_size / 1024 / 1024:.1f} MB)")
    print(f"✓ Config found: {config_path}\n")
    
    # Connect to Windows
    print(f"Connecting to {WINDOWS_HOST}...")
    ssh = create_ssh_client()
    
    # Try multiple authentication methods
    connected = False
    
    # Method 1: Try explicit key files
    key_files = [
        Path.home() / ".ssh" / "id_rsa",
        Path.home() / ".ssh" / "id_ed25519",
        Path.home() / ".ssh" / "id_DO3",
    ]
    
    for key_file in key_files:
        if not key_file.exists():
            continue
        
        print(f"  Trying key: {key_file.name}...")
        try:
            ssh.connect(
                WINDOWS_HOST,
                port=WINDOWS_PORT,
                username=WINDOWS_USER,
                key_filename=str(key_file),
                timeout=10,
                look_for_keys=False,
                allow_agent=False
            )
            print(f"✓ Connected via {key_file.name}\n")
            connected = True
            break
        except Exception as e:
            print(f"    Failed: {e}")
            continue
    
    # Method 2: Try password from environment
    if not connected and os.getenv('WINDOWS_PASSWORD'):
        print("  Trying password from WINDOWS_PASSWORD env var...")
        try:
            ssh.connect(
                WINDOWS_HOST,
                port=WINDOWS_PORT,
                username=WINDOWS_USER,
                password=os.getenv('WINDOWS_PASSWORD'),
                timeout=10,
                look_for_keys=False,
                allow_agent=False
            )
            print("✓ Connected via password\n")
            connected = True
        except Exception as e:
            print(f"    Failed: {e}")
    
    if not connected:
        print("\n❌ All authentication methods failed!")
        print("\nTry setting password:")
        print("  export WINDOWS_PASSWORD='your_password'")
        print("  python3 scripts/deploy_windows.py")
        return False
    
    try:
        # Step 1: Stop existing daemon
        print("1. Stopping existing daemon...")
        execute_command(ssh, "taskkill /F /IM flip2d.exe 2>nul || echo Not running")
        time.sleep(2)
        
        # Step 2: Create directory structure
        print("\n2. Creating directory structure...")
        directories = [
            "C:\\flip2",
            "C:\\flip2\\pb_data",
            "C:\\flip2\\archives",
            "C:\\flip2\\archives\\signals"
        ]
        for directory in directories:
            execute_command(ssh, f'mkdir "{directory}" 2>nul || echo Directory exists')
        
        # Step 3: Upload binary
        print("\n3. Uploading binary...")
        sftp = ssh.open_sftp()
        try:
            # Backup old binary
            try:
                execute_command(ssh, "copy C:\\flip2\\flip2d.exe C:\\flip2\\flip2d-backup.exe 2>nul")
            except:
                pass
            
            # Upload new binary
            remote_temp = "C:/flip2/flip2d-new.exe"
            print(f"  Uploading {binary_path.name} to {remote_temp}...")
            sftp.put(str(binary_path), remote_temp)
            print("  ✓ Upload complete")
            
            # Move to final location
            execute_command(ssh, "move /Y C:\\flip2\\flip2d-new.exe C:\\flip2\\flip2d.exe")
            
        finally:
            sftp.close()
        
        # Step 4: Upload config
        print("\n4. Uploading config...")
        sftp = ssh.open_sftp()
        try:
            print(f"  Uploading {config_path.name} to C:/flip2/config.yaml...")
            sftp.put(str(config_path), "C:/flip2/config.yaml")
            print("  ✓ Config uploaded")
        finally:
            sftp.close()
        
        # Step 5: Start daemon
        print("\n5. Starting daemon...")
        execute_command(ssh, 
            'cd C:\\flip2 && set FLIP2_ENV=production && start /B flip2d.exe --config config.yaml',
            timeout=10)
        
        print("\n6. Waiting for daemon to start...")
        time.sleep(5)
        
        # Step 7: Verify
        print("\n7. Verifying deployment...")
        import urllib.request
        try:
            response = urllib.request.urlopen(f"http://{WINDOWS_HOST}:8090/api/health", timeout=10)
            data = response.read()
            if response.status == 200:
                print("✓ Windows daemon is healthy!")
                print(f"  API: http://{WINDOWS_HOST}:8090")
                print(f"  Response: {data.decode('utf-8')[:100]}")
                return True
            else:
                print(f"⚠ Health check returned status {response.status}")
                return False
        except Exception as e:
            print(f"❌ Health check failed: {e}")
            print("\nCheck logs manually:")
            print(f"  ssh {WINDOWS_USER}@{WINDOWS_HOST} 'type C:\\flip2\\flip2d.log'")
            return False
        
    except Exception as e:
        print(f"\n❌ Deployment failed: {e}")
        import traceback
        traceback.print_exc()
        return False
    
    finally:
        ssh.close()
        print("\n=== Deployment Complete ===")

if __name__ == "__main__":
    success = deploy_to_windows()
    sys.exit(0 if success else 1)
