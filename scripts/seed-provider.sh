#!/usr/bin/env bash
# 把一条样例 LLM provider(flatkey-router)种进 Nacos 配置中心的 "shared" namespace,
# 让 Forge 的 provider 目录(/api/llm-providers)开箱就有一条真实条目可浏览,agent 也能直接引用。
#
# SHAPE ONLY —— base_url 是占位符 <ROUTER_BASE_URL>(不是真实路由地址);auth_key 只存
# 密钥的 KEY 名(ROUTER_API_KEY),真值由各 agent 在派发时从自己的 custom_env 注入,
# 任何 secret / 真实 base_url 都永不写进 Nacos。
# publish 是 UPSERT(同 dataId 直接覆盖),所以重复运行幂等,无需处理 409。
#
# Usage:
#   NACOS_SERVER_ADDR=http://127.0.0.1:8848 bash scripts/seed-provider.sh
set -euo pipefail

ADDR="${NACOS_SERVER_ADDR:-http://127.0.0.1:8848}"
IDV="${NACOS_AUTH_IDENTITY_VALUE:-nacos}"
NS="shared"
NAME="flatkey-router"
GROUP="forge-llm-providers"

SPEC=$(cat <<'JSON'
{
  "name": "flatkey-router",
  "version": "1.0.0",
  "protocol": "anthropic",
  "base_url": "<ROUTER_BASE_URL>",
  "auth_key": "ROUTER_API_KEY",
  "lifecycle": "published",
  "models": [
    { "id": "claude-sonnet-4-6", "label": "Claude Sonnet 4.6", "default": true },
    { "id": "claude-opus-4-6", "label": "Claude Opus 4.6" },
    { "id": "gpt-5.5", "label": "GPT-5.5" }
  ]
}
JSON
)

code=$(curl -s -o /tmp/seed-provider.out -w "%{http_code}" -X POST "$ADDR/nacos/v3/admin/cs/config" \
  -H "nacos: $IDV" \
  --data-urlencode "dataId=$NAME" \
  --data-urlencode "groupName=$GROUP" \
  --data-urlencode "namespaceId=$NS" \
  --data-urlencode "type=json" \
  --data-urlencode "content=$SPEC")

if [ "$code" = "200" ]; then
  echo "seeded $NAME into '$NS' namespace (group=$GROUP)"
else
  echo "seed failed (http $code): $(cat /tmp/seed-provider.out)" >&2
  exit 1
fi
