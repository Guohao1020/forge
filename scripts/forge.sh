#!/usr/bin/env bash
# forge — CLI helper for common development operations
# Usage: bash scripts/forge.sh <command>

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

case "${1:-help}" in
  status)
    echo "=== Forge Platform Status ==="
    curl -s http://localhost:8080/health 2>/dev/null | python3 -m json.tool 2>/dev/null || echo "API: not running"
    curl -s http://localhost:8080/api/system/info 2>/dev/null | python3 -m json.tool 2>/dev/null || true
    echo ""
    echo "Docker containers:"
    docker compose -f "$ROOT_DIR/docker-compose.dev.yml" ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "  (docker not available)"
    ;;

  logs)
    SERVICE="${2:-forge-core}"
    case "$SERVICE" in
      core|forge-core)
        echo "=== forge-core logs (last 50 lines) ==="
        docker logs forge-core --tail 50 2>/dev/null || echo "Container not running. Run locally with: cd forge-core && go run ./cmd/forge-core"
        ;;
      bot|forge-bot)
        docker logs forge-bot --tail 50 2>/dev/null || echo "Container not running"
        ;;
      *)
        docker logs "forge-$SERVICE" --tail 50 2>/dev/null || docker logs "$SERVICE" --tail 50 2>/dev/null || echo "Unknown service: $SERVICE"
        ;;
    esac
    ;;

  test)
    echo "=== Running all tests ==="
    echo "--- Go (forge-core) ---"
    cd "$ROOT_DIR/forge-core" && go test ./internal/... -count=1 | grep -E "^ok|^FAIL"
    echo "--- Go (forge-bot) ---"
    cd "$ROOT_DIR/forge-bot" && go test ./... -count=1 | grep -E "^ok|^FAIL"
    echo "--- TypeScript ---"
    cd "$ROOT_DIR/forge-portal" && npx tsc --noEmit && echo "ok  TypeScript check"
    echo "=== Done ==="
    ;;

  db)
    echo "Connecting to PostgreSQL..."
    docker exec -it forge-postgres psql -U forge forge_main
    ;;

  redis)
    echo "Connecting to Redis..."
    docker exec -it forge-redis redis-cli -a forge_redis_2026
    ;;

  migrate)
    echo "Running migrations..."
    cd "$ROOT_DIR/forge-core" && go run ./cmd/forge-core
    ;;

  count)
    echo "=== Codebase Stats ==="
    echo "Go files: $(find "$ROOT_DIR/forge-core" "$ROOT_DIR/forge-bot" -name '*.go' ! -path '*/workspaces/*' | wc -l)"
    echo "Go tests: $(cd "$ROOT_DIR/forge-core" && go test ./internal/... -count=1 -v 2>&1 | grep -cE '^--- PASS') (forge-core)"
    echo "Python files: $(find "$ROOT_DIR/ai-worker" -name '*.py' | wc -l)"
    echo "TypeScript files: $(find "$ROOT_DIR/forge-portal/app" "$ROOT_DIR/forge-portal/components" "$ROOT_DIR/forge-portal/lib" -name '*.tsx' -o -name '*.ts' | wc -l)"
    echo "Frontend pages: $(find "$ROOT_DIR/forge-portal/app" -name 'page.tsx' | wc -l)"
    echo "Components: $(find "$ROOT_DIR/forge-portal/components" -name '*.tsx' | wc -l)"
    echo "Migrations: $(ls "$ROOT_DIR/forge-core/migrations/"*.sql | wc -l)"
    echo "Git tags: $(git -C "$ROOT_DIR" tag | wc -l)"
    ;;

  help|*)
    echo "Usage: bash scripts/forge.sh <command>"
    echo ""
    echo "Commands:"
    echo "  status    Show platform health and docker status"
    echo "  logs      View service logs (core, bot, postgres, etc.)"
    echo "  test      Run all test suites"
    echo "  db        Connect to PostgreSQL via psql"
    echo "  redis     Connect to Redis CLI"
    echo "  count     Show codebase statistics"
    echo "  help      Show this help"
    ;;
esac
