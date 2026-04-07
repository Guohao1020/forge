#!/bin/sh
# Entry point for ai-worker container.
# Starts FastAPI (uvicorn on :8090) and Temporal worker side-by-side.
# Forwards SIGTERM/SIGINT to both children so `docker stop` exits cleanly.

set -e

# Start FastAPI in background
uvicorn src.api_server:app --host 0.0.0.0 --port 8090 &
API_PID=$!

# Start Temporal worker in background
python -B -m src.worker &
WORKER_PID=$!

# Forward signals to both children
term_handler() {
  kill -TERM "$API_PID" 2>/dev/null || true
  kill -TERM "$WORKER_PID" 2>/dev/null || true
  wait "$API_PID" 2>/dev/null || true
  wait "$WORKER_PID" 2>/dev/null || true
  exit 0
}
trap term_handler TERM INT

# If either child dies, exit with its status so the container restarts
wait -n "$API_PID" "$WORKER_PID"
EXIT_CODE=$?
kill -TERM "$API_PID" "$WORKER_PID" 2>/dev/null || true
exit $EXIT_CODE
