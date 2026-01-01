#!/bin/bash
# Transfer FLIP2 Windows deployment package to Windows machine
# Usage: ./transfer.sh [windows_user@windows_ip]

WINDOWS_HOST="${1:-ariel@192.168.1.220}"
REMOTE_DIR="C:/flip2"

echo "=== FLIP2 Windows Deployment Transfer ==="
echo "Target: $WINDOWS_HOST:$REMOTE_DIR"
echo ""

# Check if Windows is reachable
echo "Checking Windows connectivity..."
ping -c 1 192.168.1.220 > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "ERROR: Windows machine not reachable at 192.168.1.220"
    exit 1
fi

echo "Windows is reachable."
echo ""

# Transfer files via SCP
echo "Transferring files..."
scp flip2d.exe flip2.exe config.yaml DEPLOY.md "$WINDOWS_HOST:$REMOTE_DIR/"

if [ $? -eq 0 ]; then
    echo ""
    echo "=== Transfer Complete ==="
    echo ""
    echo "Next steps on Windows:"
    echo "  1. Open PowerShell as Administrator"
    echo "  2. cd C:\\flip2"
    echo "  3. .\\flip2d.exe --config config.yaml --foreground"
    echo ""
    echo "Or read DEPLOY.md for full instructions."
else
    echo ""
    echo "Transfer failed. Try manual copy:"
    echo "  1. Share deploy/windows folder"
    echo "  2. Copy contents to C:\\flip2 on Windows"
fi
