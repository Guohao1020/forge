#!/bin/bash
# Stop all Forge development services
set -euo pipefail
cd "$(dirname "$0")/.."

echo "Stopping forge-core..."
taskkill //F //IM forge-core.exe 2>/dev/null || pkill -f forge-core 2>/dev/null || true

echo "Stopping AI Worker..."
taskkill //F //IM python.exe 2>/dev/null || pkill -f "src.worker" 2>/dev/null || true

echo "Stopping Next.js..."
taskkill //F //IM node.exe 2>/dev/null || pkill -f "next dev" 2>/dev/null || true

echo "Stopping infrastructure..."
docker compose -f docker-compose.dev.yml down 2>/dev/null || true

echo "All services stopped."
