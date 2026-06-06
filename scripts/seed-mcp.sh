#!/usr/bin/env bash
# Seed a sample MCP server into the Nacos AI Registry "shared" namespace so the
# Forge MCP catalog (/api/mcp-registry/servers) has a real entry to browse and
# agents can reference out of the box.
#
# SHAPE ONLY — env_keys names the secret the agent must supply via its own
# custom_env (e.g. VOC_API_KEY); no secret value is ever stored in Nacos.
# Register is create-only, so a re-run on an already-seeded server is a no-op.
#
# Usage:
#   NACOS_SERVER_ADDR=http://127.0.0.1:8848 scripts/seed-mcp.sh
set -euo pipefail

ADDR="${NACOS_SERVER_ADDR:-http://127.0.0.1:8848}"
IDV="${NACOS_AUTH_IDENTITY_VALUE:-nacos}"
NS="shared"
NAME="voc-openapi"

SPEC=$(cat <<'JSON'
{
  "name": "voc-openapi",
  "protocol": "stdio",
  "description": "Amazon VOC / review-analytics MCP. Shape only — set VOC_API_KEY in each agent's environment; the catalog records the KEY name, never the value.",
  "versionDetail": { "version": "1.0.0" },
  "localServerConfig": {
    "command": "voc-mcp",
    "args": ["--stdio"],
    "env_keys": ["VOC_API_KEY"]
  }
}
JSON
)

code=$(curl -s -o /tmp/seed-mcp.out -w "%{http_code}" -X POST "$ADDR/nacos/v3/admin/ai/mcp" \
  -H "nacos: $IDV" \
  --data-urlencode "namespaceId=$NS" \
  --data-urlencode "mcpName=$NAME" \
  --data-urlencode "serverSpecification=$SPEC")

if [ "$code" = "200" ]; then
  echo "seeded $NAME into '$NS' namespace"
elif grep -qiE 'already exist|exists' /tmp/seed-mcp.out 2>/dev/null; then
  echo "$NAME already present in '$NS' — no-op"
else
  echo "seed failed (http $code): $(cat /tmp/seed-mcp.out)" >&2
  exit 1
fi
