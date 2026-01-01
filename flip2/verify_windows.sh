#!/bin/bash
# Verify Windows FLIP2 deployment from Mac

echo "=== Verifying Windows FLIP2 Deployment ==="
echo

echo "1. Testing Windows health endpoint..."
if curl -s -f http://192.168.1.220:8090/api/health -H "X-API-Key: flip2_secret_key_123" | grep -q "healthy"; then
    echo "   ✅ Windows daemon is healthy"
else
    echo "   ❌ Windows daemon health check failed"
fi

echo
echo "2. Testing Mac → Windows sync..."
SIGNAL_ID="test-sync-$(date +%s)"
curl -s -k -X POST https://localhost:8090/api/collections/signals/records \
  -H "X-API-Key: flip2_secret_key_123" \
  -H "Content-Type: application/json" \
  -d "{
    \"signal_id\": \"$SIGNAL_ID\",
    \"from_agent\": \"mac\",
    \"to_agent\": \"windows\",
    \"signal_type\": \"message\",
    \"content\": \"Test sync from Mac\"
  }" > /dev/null

echo "   Sent signal: $SIGNAL_ID"
echo "   Waiting 30 seconds for sync..."
sleep 30

if curl -s "http://192.168.1.220:8090/api/collections/signals/records?filter=signal_id='$SIGNAL_ID'" \
  -H "X-API-Key: flip2_secret_key_123" | grep -q "$SIGNAL_ID"; then
    echo "   ✅ Signal synced to Windows successfully"
else
    echo "   ❌ Signal not found on Windows (may need more time)"
fi

echo
echo "3. Checking Windows daemon status..."
echo "   Run on Windows: tasklist | findstr flip2d"
echo "   Run on Windows: netstat -ano | findstr :8090"
echo
echo "=== Verification Complete ==="
