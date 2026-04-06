#!/usr/bin/env bash
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

BASE_URL="http://localhost:8080"
PROJECT_ID=1604

echo -e "${GREEN}=== Forge Smoke Test ===${NC}"
echo ""

# Step 0: Health check
echo -e "${YELLOW}[0] Health check...${NC}"
HEALTH=$(curl -sf "$BASE_URL/health" 2>/dev/null || echo '{"status":"error"}')
echo "  $HEALTH"

# Step 1: Login
echo -e "${YELLOW}[1] Login...${NC}"
TOKEN=$(curl -sf -X POST "$BASE_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.loads(sys.stdin.read())['data']['token'])")
echo -e "  ${GREEN}✓${NC} Token obtained"

AUTH="Authorization: Bearer $TOKEN"

# Step 2: Create task
echo -e "${YELLOW}[2] Creating task...${NC}"
TASK_RESP=$(curl -sf -X POST "$BASE_URL/api/projects/$PROJECT_ID/tasks" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"title":"Smoke test task","requirement":"Add a hello world endpoint that returns JSON {\"message\": \"hello from forge\"}"}')
TASK_ID=$(echo "$TASK_RESP" | python3 -c "
import sys, json, re, io
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
raw = sys.stdin.buffer.read().decode('utf-8', errors='replace')
raw = re.sub(r'[\x00-\x1f](?![\"\\\\\/bfnrt])', ' ', raw)
data = json.loads(raw)
print(data['data']['task']['id'])
")
echo -e "  ${GREEN}✓${NC} Task #$TASK_ID created"

# Step 3: Send message (triggers analysis)
echo -e "${YELLOW}[3] Sending message (triggers AI analysis)...${NC}"
curl -sf -X POST "$BASE_URL/api/projects/$PROJECT_ID/tasks/$TASK_ID/messages" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"content":"Just do it as described, confirm directly please"}' > /dev/null
echo -e "  ${GREEN}✓${NC} Message sent, waiting for AI analysis..."

# Step 4: Wait for confirmed status
echo -e "${YELLOW}[4] Waiting for analysis to complete (max 3 min)...${NC}"
CONFIRMED=false
for i in $(seq 1 36); do
  sleep 5
  MSGS=$(curl -sf -H "$AUTH" "$BASE_URL/api/projects/$PROJECT_ID/tasks/$TASK_ID/messages" 2>/dev/null || echo '{}')
  STATUS=$(echo "$MSGS" | python3 -c "
import sys, json, re, io
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
raw = sys.stdin.buffer.read().decode('utf-8', errors='replace')
raw = re.sub(r'[\x00-\x1f](?![\"\\\\\/bfnrt])', ' ', raw)
try:
    data = json.loads(raw)
    msgs = data.get('data', {}).get('messages', [])
    for m in reversed(msgs):
        meta = m.get('metadata')
        if isinstance(meta, dict) and meta.get('status') == 'confirmed':
            print('confirmed')
            break
    else:
        print('waiting')
except:
    print('error')
" 2>/dev/null)
  if [ "$STATUS" = "confirmed" ]; then
    CONFIRMED=true
    echo -e "  ${GREEN}✓${NC} Requirements confirmed"
    break
  fi
  echo -n "."
done
echo ""

if [ "$CONFIRMED" != "true" ]; then
  # Try sending another message to push for confirmation
  curl -sf -X POST "$BASE_URL/api/projects/$PROJECT_ID/tasks/$TASK_ID/messages" \
    -H "$AUTH" -H "Content-Type: application/json" \
    -d '{"content":"Yes, confirmed. Please proceed."}' > /dev/null
  for i in $(seq 1 24); do
    sleep 5
    MSGS=$(curl -sf -H "$AUTH" "$BASE_URL/api/projects/$PROJECT_ID/tasks/$TASK_ID/messages" 2>/dev/null || echo '{}')
    STATUS=$(echo "$MSGS" | python3 -c "
import sys, json, re, io
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
raw = sys.stdin.buffer.read().decode('utf-8', errors='replace')
raw = re.sub(r'[\x00-\x1f](?![\"\\\\\/bfnrt])', ' ', raw)
try:
    data = json.loads(raw)
    msgs = data.get('data', {}).get('messages', [])
    for m in reversed(msgs):
        meta = m.get('metadata')
        if isinstance(meta, dict) and meta.get('status') == 'confirmed':
            print('confirmed')
            break
    else:
        print('waiting')
except:
    print('error')
" 2>/dev/null)
    if [ "$STATUS" = "confirmed" ]; then
      CONFIRMED=true
      echo -e "  ${GREEN}✓${NC} Requirements confirmed (2nd attempt)"
      break
    fi
    echo -n "."
  done
  echo ""
fi

if [ "$CONFIRMED" != "true" ]; then
  echo -e "  ${RED}✗${NC} Analysis did not confirm within timeout"
  exit 1
fi

# Step 5: Confirm plan
echo -e "${YELLOW}[5] Confirming plan (triggers plan generation)...${NC}"
curl -sf -X POST "$BASE_URL/api/projects/$PROJECT_ID/tasks/$TASK_ID/confirm" \
  -H "$AUTH" -H "Content-Type: application/json" > /dev/null
echo -e "  ${GREEN}✓${NC} Plan generation started, waiting..."

# Step 6: Wait for PLAN step COMPLETED
echo -e "${YELLOW}[6] Waiting for plan to complete (max 3 min)...${NC}"
PLAN_DONE=false
for i in $(seq 1 36); do
  sleep 5
  TASK_DATA=$(curl -sf -H "$AUTH" "$BASE_URL/api/projects/$PROJECT_ID/tasks/$TASK_ID" 2>/dev/null)
  PLAN_STATUS=$(echo "$TASK_DATA" | python3 -c "
import sys, json, re, io
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
raw = sys.stdin.buffer.read().decode('utf-8', errors='replace')
raw = re.sub(r'[\x00-\x1f](?![\"\\\\\/bfnrt])', ' ', raw)
data = json.loads(raw)['data']
for s in data.get('steps', []):
    if s['step_type'] == 'PLAN':
        print(s['status'])
        break
" 2>/dev/null)
  if [ "$PLAN_STATUS" = "COMPLETED" ]; then
    PLAN_DONE=true
    echo -e "  ${GREEN}✓${NC} Plan generated"
    break
  fi
  echo -n "."
done
echo ""

if [ "$PLAN_DONE" != "true" ]; then
  echo -e "  ${RED}✗${NC} Plan generation timed out"
  exit 1
fi

# Step 7: Approve plan (triggers execution)
echo -e "${YELLOW}[7] Approving plan (triggers code generation)...${NC}"
curl -sf -X POST "$BASE_URL/api/projects/$PROJECT_ID/tasks/$TASK_ID/approve-plan" \
  -H "$AUTH" -H "Content-Type: application/json" > /dev/null
echo -e "  ${GREEN}✓${NC} Execution started, waiting..."

# Step 8: Wait for COMPLETED
echo -e "${YELLOW}[8] Waiting for task to complete (max 5 min)...${NC}"
TASK_DONE=false
for i in $(seq 1 60); do
  sleep 5
  TASK_DATA=$(curl -sf -H "$AUTH" "$BASE_URL/api/projects/$PROJECT_ID/tasks/$TASK_ID" 2>/dev/null)
  TASK_STATUS=$(echo "$TASK_DATA" | python3 -c "
import sys, json, re, io
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
raw = sys.stdin.buffer.read().decode('utf-8', errors='replace')
raw = re.sub(r'[\x00-\x1f](?![\"\\\\\/bfnrt])', ' ', raw)
data = json.loads(raw)['data']
print(data['task']['status'])
" 2>/dev/null)
  if [ "$TASK_STATUS" = "COMPLETED" ]; then
    TASK_DONE=true
    break
  elif [ "$TASK_STATUS" = "FAILED" ]; then
    echo -e "\n  ${RED}✗${NC} Task FAILED"
    exit 1
  fi
  echo -n "."
done
echo ""

if [ "$TASK_DONE" != "true" ]; then
  echo -e "  ${RED}✗${NC} Task did not complete within timeout"
  exit 1
fi

# Step 9: Print final results
echo ""
echo -e "${GREEN}=== SMOKE TEST PASSED ===${NC}"
echo ""
echo "$TASK_DATA" | python3 -c "
import sys, json, re, io
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
raw = sys.stdin.buffer.read().decode('utf-8', errors='replace')
raw = re.sub(r'[\x00-\x1f](?![\"\\\\\/bfnrt])', ' ', raw)
data = json.loads(raw)['data']
task = data['task']
print(f'Task #{task[\"id\"]}: {task[\"status\"]}')
print(f'PR URL: {task.get(\"pr_url\", \"(none)\")}')
print()
for s in data.get('steps', []):
    output_len = len(s.get('output','') or '') if s.get('output') else 0
    mock = ''
    if s['step_type'] == 'TEST' and s.get('output'):
        try:
            o = json.loads(s['output'])
            mock = ' (mock)' if o.get('mock') else ' (real)'
        except: pass
    print(f'  {s[\"step_type\"]:15s} {s[\"status\"]:10s} ({output_len} chars){mock}')
" 2>/dev/null
