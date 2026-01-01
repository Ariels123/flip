#!/bin/bash
# start_chat_loop.sh

echo "Starting FLIP2 Chat Loop..."
echo "Press Ctrl+C to stop."

while true; do
  echo "[$(date +%T)] Polling Claude..."
  ./flip2 agent poll claude
  
  echo "[$(date +%T)] Polling Gemini..."
  ./flip2 agent poll gemini
  
  sleep 5
done
