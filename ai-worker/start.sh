#!/bin/sh
# Entry point for ai-worker container.
# Starts FastAPI (uvicorn on :8090) and Temporal worker side-by-side.
# POSIX-only (no bashisms): runs in dash inside python:3.12-slim.
# Forwards SIGTERM/SIGINT to both children, exits when either child dies
# so the container restart policy can recover.

set -e

# Start FastAPI in background
uvicorn src.api_server:app --host 0.0.0.0 --port 8090 &
API_PID=$!

# Start Temporal worker in background
python -B -m src.worker &
WORKER_PID=$!

# Forward signals to both children, then exit cleanly.
term_handler() {
  kill -TERM "$API_PID" 2>/dev/null || true
  kill -TERM "$WORKER_PID" 2>/dev/null || true
  wait "$API_PID" 2>/dev/null || true
  wait "$WORKER_PID" 2>/dev/null || true
  exit 0
}
trap term_handler TERM INT

# Poll children. As soon as either dies, propagate its status so docker
# restart policy kicks in. dash does not support `wait -n`, so we sleep+poll.
while :; do
  if ! kill -0 "$API_PID" 2>/dev/null; then
    wait "$API_PID"
    EXIT_CODE=$?
    echo "start.sh: uvicorn (pid $API_PID) exited with $EXIT_CODE" >&2
    kill -TERM "$WORKER_PID" 2>/dev/null || true
    wait "$WORKER_PID" 2>/dev/null || true
    exit "$EXIT_CODE"
  fi
  if ! kill -0 "$WORKER_PID" 2>/dev/null; then
    wait "$WORKER_PID"
    EXIT_CODE=$?
    echo "start.sh: temporal worker (pid $WORKER_PID) exited with $EXIT_CODE" >&2
    kill -TERM "$API_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
    exit "$EXIT_CODE"
  fi
  sleep 2
done
