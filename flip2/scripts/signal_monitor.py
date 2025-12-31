#!/usr/bin/env python3
"""
FLIP2 Signal Monitor with Auto-ACK
Polls for new signals and sends immediate acknowledgments.

Usage:
    python3 signal_monitor.py [--interval 2] [--agent claude-mac]
"""

import argparse
import json
import os
import sys
import time
import urllib.request
import urllib.error
import ssl
from datetime import datetime

# Configuration
DEFAULT_SERVER = "https://localhost:8091"
DEFAULT_API_KEY = "flip2_secret_key_123"
DEFAULT_AGENT = "claude-mac"
DEFAULT_INTERVAL = 2  # seconds

# Create SSL context that ignores self-signed certs
SSL_CONTEXT = ssl.create_default_context()
SSL_CONTEXT.check_hostname = False
SSL_CONTEXT.verify_mode = ssl.CERT_NONE

class SignalMonitor:
    def __init__(self, server, api_key, agent_id, interval):
        self.server = server
        self.api_key = api_key
        self.agent_id = agent_id
        self.interval = interval
        self.seen_signals = set()
        self.running = True

    def log(self, msg):
        timestamp = datetime.now().strftime("%H:%M:%S")
        print(f"[{timestamp}] {msg}")
        sys.stdout.flush()

    def api_request(self, method, endpoint, data=None):
        """Make API request with proper headers."""
        url = f"{self.server}{endpoint}"
        headers = {
            "X-API-Key": self.api_key,
        }
        if data:
            headers["Content-Type"] = "application/json"

        body = json.dumps(data).encode() if data else None
        req = urllib.request.Request(url, data=body, headers=headers, method=method)

        try:
            with urllib.request.urlopen(req, context=SSL_CONTEXT, timeout=10) as resp:
                return json.loads(resp.read().decode())
        except urllib.error.HTTPError as e:
            # Read error body for details
            try:
                err_body = e.read().decode()
                self.log(f"HTTP Error {e.code}: {err_body[:100]}")
            except:
                self.log(f"HTTP Error {e.code}: {e.reason}")
            return None
        except Exception as e:
            self.log(f"Request error: {e}")
            return None

    def get_new_signals(self):
        """Fetch unread signals for this agent."""
        # Use pre-encoded filter (PocketBase is picky about encoding)
        # filter: to_agent='X' && read=false
        agent_encoded = self.agent_id.replace("'", "%27")
        filter_part = f"to_agent%3D%27{agent_encoded}%27%20%26%26%20read%3Dfalse"
        endpoint = f"/api/collections/signals/records?filter={filter_part}&perPage=50"
        result = self.api_request("GET", endpoint)

        if result and "items" in result:
            return result["items"]
        return []

    def mark_as_read(self, signal_id):
        """Mark a signal as read."""
        endpoint = f"/api/collections/signals/records/{signal_id}"
        data = {
            "read": True,
            "read_at": datetime.utcnow().isoformat() + "Z"
        }
        return self.api_request("PATCH", endpoint, data)

    def send_ack(self, original_signal):
        """Send acknowledgment for a received signal."""
        signal_id = original_signal.get("signal_id", original_signal.get("id", "unknown"))
        from_agent = original_signal.get("from_agent", "unknown")

        # Don't ACK our own signals or ACK signals
        if from_agent == self.agent_id:
            return
        if "ACK-" in signal_id or "-ACK-" in signal_id:
            return

        ack_data = {
            "signal_id": f"ACK-{signal_id}-{int(time.time())}",
            "from_agent": self.agent_id,
            "to_agent": from_agent,
            "content": f"ACK: Signal {signal_id} received at {datetime.now().isoformat()}",
            "priority": "normal",
            "signal_type": "message"  # Use 'message' type for ACKs
        }

        result = self.api_request("POST", "/api/collections/signals/records", ack_data)
        if result:
            self.log(f"  -> ACK sent to {from_agent}")
        return result

    def process_signal(self, signal):
        """Process a single signal."""
        signal_id = signal.get("signal_id", signal.get("id", "unknown"))
        from_agent = signal.get("from_agent", "unknown")
        content_preview = signal.get("content", "")[:100]

        self.log(f"NEW SIGNAL from {from_agent}: {signal_id}")
        self.log(f"  Content: {content_preview}...")

        # Mark as read
        self.mark_as_read(signal["id"])

        # Send ACK
        self.send_ack(signal)

        # Track seen signals
        self.seen_signals.add(signal["id"])

    def run(self):
        """Main monitoring loop."""
        self.log(f"Signal Monitor started for agent: {self.agent_id}")
        self.log(f"Server: {self.server}")
        self.log(f"Polling interval: {self.interval}s")
        self.log("-" * 50)

        while self.running:
            try:
                signals = self.get_new_signals()

                for signal in signals:
                    if signal["id"] not in self.seen_signals:
                        self.process_signal(signal)

                time.sleep(self.interval)

            except KeyboardInterrupt:
                self.log("Shutting down...")
                self.running = False
            except Exception as e:
                self.log(f"Error: {e}")
                time.sleep(self.interval)

def main():
    parser = argparse.ArgumentParser(description="FLIP2 Signal Monitor with Auto-ACK")
    parser.add_argument("--server", default=DEFAULT_SERVER, help="FLIP2 server URL")
    parser.add_argument("--api-key", default=DEFAULT_API_KEY, help="API key")
    parser.add_argument("--agent", default=DEFAULT_AGENT, help="Agent ID")
    parser.add_argument("--interval", type=float, default=DEFAULT_INTERVAL, help="Poll interval in seconds")

    args = parser.parse_args()

    monitor = SignalMonitor(
        server=args.server,
        api_key=args.api_key,
        agent_id=args.agent,
        interval=args.interval
    )

    monitor.run()

if __name__ == "__main__":
    main()
