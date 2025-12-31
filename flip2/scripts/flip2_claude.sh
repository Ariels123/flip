#!/bin/bash
# FLIP2 Claude Integration Helper Script
# Implements the /flip2-* skill commands

set -e

# Configuration (can be overridden by environment variables)
FLIP2_URL="${FLIP2_URL:-https://localhost:8090}"
FLIP2_API_KEY="${FLIP2_API_KEY:-flip2_secret_key_123}"
FLIP2_AGENT_ID="${FLIP2_AGENT_ID:-claude-mac}"

# Helper function for API calls
api_call() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    
    if [ -n "$data" ]; then
        curl -s -k -X "$method" \
            "${FLIP2_URL}${endpoint}" \
            -H "X-API-Key: ${FLIP2_API_KEY}" \
            -H "Content-Type: application/json" \
            -d "$data"
    else
        curl -s -k -X "$method" \
            "${FLIP2_URL}${endpoint}" \
            -H "X-API-Key: ${FLIP2_API_KEY}"
    fi
}

# Command: register
flip2_register() {
    local agent_id="${1:-$FLIP2_AGENT_ID}"
    
    echo "Registering agent: $agent_id"
    
    local data=$(cat <<EOF
{
    "agent_id": "$agent_id",
    "status": "online",
    "backend": "claude",
    "capabilities": {
        "skills": ["coding", "debugging", "analysis", "documentation"],
        "languages": ["python", "go", "javascript", "bash", "typescript"],
        "max_concurrent": 3
    },
    "last_seen": "$(date -u +%Y-%m-%dT%H:%M:%S.000Z)"
}
EOF
)
    
    response=$(api_call POST "/api/collections/agents/records" "$data")
    
    if echo "$response" | grep -q '"id"'; then
        echo "✓ Agent registered successfully"
        echo "$response" | python3 -m json.tool
    else
        echo "✗ Registration failed"
        echo "$response"
        return 1
    fi
}

# Command: status
flip2_status() {
    local status="$1"
    local reason="$2"
    
    if [ -z "$status" ]; then
        echo "Usage: flip2_status <online|offline|busy|idle> [reason]"
        return 1
    fi
    
    echo "Updating status to: $status"
    
    # First, find the agent record
    local agent_response=$(api_call GET "/api/collections/agents/records?filter=agent_id='${FLIP2_AGENT_ID}'")
    local record_id=$(echo "$agent_response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['items'][0]['id'] if data.get('items') else '')")
    
    if [ -z "$record_id" ]; then
        echo "✗ Agent not found. Please register first with: flip2_register"
        return 1
    fi
    
    local data=$(cat <<EOF
{
    "status": "$status",
    "last_seen": "$(date -u +%Y-%m-%dT%H:%M:%S.000Z)"
}
EOF
)
    
    response=$(api_call PATCH "/api/collections/agents/records/$record_id" "$data")
    echo "✓ Status updated"
}

# Command: inbox
flip2_inbox() {
    echo "Checking inbox for: $FLIP2_AGENT_ID"
    
    local response=$(api_call GET "/api/collections/signals/records?filter=(to_agent='${FLIP2_AGENT_ID}'%26%26read=false)&sort=-created")
    
    local count=$(echo "$response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data.get('items', [])))")
    
    if [ "$count" = "0" ]; then
        echo "📭 No new signals"
    else
        echo "📬 $count new signal(s):"
        echo ""
        echo "$response" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for i, item in enumerate(data.get('items', []), 1):
    print(f\"{i}. [{item.get('priority', 'normal').upper()}] From: {item.get('from_agent')} ({item.get('signal_type')})\")
    print(f\"   ID: {item.get('signal_id')}\")
    print(f\"   Message: {item.get('content', '')[:100]}\")
    print()
"
    fi
}

# Command: send
flip2_send() {
    local to_agent="$1"
    shift
    local message="$*"
    
    if [ -z "$to_agent" ] || [ -z "$message" ]; then
        echo "Usage: flip2_send <to-agent> <message>"
        return 1
    fi
    
    echo "Sending signal to: $to_agent"
    
    local signal_id="sig-$(date +%s)-$(( RANDOM % 1000 ))"
    local data=$(cat <<EOF
{
    "signal_id": "$signal_id",
    "from_agent": "$FLIP2_AGENT_ID",
    "to_agent": "$to_agent",
    "signal_type": "message",
    "priority": "normal",
    "content": "$message",
    "read": false
}
EOF
)
    
    response=$(api_call POST "/api/collections/signals/records" "$data")
    
    if echo "$response" | grep -q '"id"'; then
        echo "✓ Signal sent (ID: $signal_id)"
    else
        echo "✗ Send failed"
        echo "$response"
        return 1
    fi
}

# Command: health
flip2_health() {
    echo "Checking FLIP2 health..."
    
    response=$(curl -s -k "${FLIP2_URL}/api/health")
    
    if echo "$response" | grep -q '"message":"API is healthy"'; then
        echo "✓ FLIP2 is healthy"
        echo "$response" | python3 -m json.tool
    else
        echo "✗ FLIP2 health check failed"
        echo "$response"
        return 1
    fi
}

# Command: agents
flip2_agents() {
    echo "Listing all agents..."
    
    local response=$(api_call GET "/api/collections/agents/records?sort=-last_seen")
    
    echo "$response" | python3 -c "
import sys, json
from datetime import datetime

data = json.load(sys.stdin)
agents = data.get('items', [])

if not agents:
    print('No agents found')
else:
    print(f'Total agents: {len(agents)}\n')
    for agent in agents:
        agent_id = agent.get('agent_id', 'unknown')
        status = agent.get('status', 'unknown')
        backend = agent.get('backend', 'unknown')
        last_seen = agent.get('last_seen', '')
        
        status_icon = '🟢' if status == 'online' else '🔴' if status == 'offline' else '🟡'
        
        print(f'{status_icon} {agent_id} ({backend})')
        print(f'   Status: {status}')
        if last_seen:
            print(f'   Last seen: {last_seen}')
        print()
"
}

# Command: config
flip2_config() {
    echo "FLIP2 Configuration:"
    echo "  URL:       $FLIP2_URL"
    echo "  Agent ID:  $FLIP2_AGENT_ID"
    echo "  API Key:   ${FLIP2_API_KEY:0:20}... (hidden)"
}

# Main command dispatcher
case "$1" in
    register)
        shift
        flip2_register "$@"
        ;;
    status)
        shift
        flip2_status "$@"
        ;;
    inbox)
        flip2_inbox
        ;;
    send)
        shift
        flip2_send "$@"
        ;;
    health)
        flip2_health
        ;;
    agents)
        flip2_agents
        ;;
    config)
        flip2_config
        ;;
    *)
        echo "FLIP2 Claude Integration Helper"
        echo ""
        echo "Usage: $0 <command> [args]"
        echo ""
        echo "Commands:"
        echo "  register [agent-id]          Register as FLIP2 agent"
        echo "  status <status> [reason]     Update agent status"
        echo "  inbox                        Check for new signals"
        echo "  send <agent> <message>       Send signal to agent"
        echo "  health                       Check FLIP2 health"
        echo "  agents                       List all agents"
        echo "  config                       Show configuration"
        echo ""
        echo "Environment variables:"
        echo "  FLIP2_URL       (default: https://localhost:8090)"
        echo "  FLIP2_API_KEY   (default: flip2_secret_key_123)"
        echo "  FLIP2_AGENT_ID  (default: claude-mac)"
        exit 1
        ;;
esac
