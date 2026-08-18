#!/usr/bin/env bash
# End-to-end test for DevCapsule.
# Requires: docker compose up -d.
set -euo pipefail

BASE="${BASE:-http://localhost}"
ADMIN_USER="${ADMIN_USERNAME:-admin}"
ADMIN_PASS="${ADMIN_PASSWORD:-admin-e2e-pass}"
PREFIX="${PREFIX:-stu}"
COUNT="${COUNT:-3}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
ADMIN_JAR="$TMP/admin.cookies"
STU_JAR="$TMP/stu.cookies"

pass=0; fail=0
ok()   { echo "  PASS $1"; pass=$((pass+1)); }
bad()  { echo "  FAIL $1"; fail=$((fail+1)); }

echo "[0] clean previous test data"
ADMIN_LOGIN_CODE=$(curl -s -c "$ADMIN_JAR" -o /dev/null -w '%{http_code}' \
  -X POST "$BASE/platform/auth/login" -H 'Content-Type: application/json' \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}")
if [ "$ADMIN_LOGIN_CODE" != "200" ]; then
  echo "  ERROR: admin login failed (HTTP $ADMIN_LOGIN_CODE); set ADMIN_USERNAME/ADMIN_PASSWORD to match this platform" >&2
  exit 1
fi
# remove all non-admin users (drops their containers + access logs)
for uid in $(curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" "$BASE/platform/admin/users" | jq -r '.data[] | select(.role!="admin") | .id'); do
  curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" -o /dev/null -X DELETE "$BASE/platform/admin/users/$uid" || true
done
# remove the e2e template if present
TPL_ID=$(curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" "$BASE/platform/admin/templates" | jq -r '.data[] | select(.name=="e2e-student") | .id')
if [ -n "$TPL_ID" ]; then curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" -o /dev/null -X DELETE "$BASE/platform/admin/templates/$TPL_ID"; fi
ok "cleanup done"

echo "[1] admin login"
curl -s -c "$ADMIN_JAR" -o "$TMP/login.json" -w '%{http_code}' \
  -X POST "$BASE/platform/auth/login" -H 'Content-Type: application/json' \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" | grep -q 200 && ok "admin login 200" || bad "admin login"
ADMIN_ROLE=$(jq -r '.data.user.role // empty' "$TMP/login.json")
[ "$ADMIN_ROLE" = "admin" ] && ok "admin role=admin" || bad "admin role got '$ADMIN_ROLE'"

echo "[2] create template"
curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" -o "$TMP/tpl.json" -w '%{http_code}' \
  -X POST "$BASE/platform/admin/templates" -H 'Content-Type: application/json' \
  -d '{"name":"e2e-student","image":"ghcr.io/anomalyco/opencode:latest","internal_port":4096,"extra_ports":[3000],"cpu_limit":0.5,"mem_limit":1073741824,"workspace_dir":"/workspace","command":["opencode","web","--hostname","0.0.0.0","--port","4096"]}' \
  | grep -q 200 && ok "template created" || bad "create template: $(cat "$TMP/tpl.json")"
TPL_ID=$(jq -r '.data.id' "$TMP/tpl.json")
EXTRA_PORTS=$(jq -r '.data.extra_ports | join(",")' "$TMP/tpl.json")
[ "$EXTRA_PORTS" = "3000" ] && ok "extra_ports persisted ($EXTRA_PORTS)" || bad "extra_ports got '$EXTRA_PORTS'"

echo "[3] batch create users (with course)"
curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" -o "$TMP/users.json" -w '%{http_code}' \
  -X POST "$BASE/platform/admin/users/batch" -H 'Content-Type: application/json' \
  -d "{\"count\":$COUNT,\"prefix\":\"$PREFIX\",\"password_length\":10,\"course\":\"E2E Course\",\"cpu_limit\":0.5,\"mem_limit\":1073741824}" \
  | grep -q 200 && ok "users created" || bad "create users: $(cat "$TMP/users.json")"
CREATED=$(jq -r '.data.created' "$TMP/users.json")
[ "$CREATED" = "$COUNT" ] && ok "created count=$COUNT" || bad "created=$CREATED"
USER_IDS=$(jq -c '[.data.users[].id]' "$TMP/users.json")
FIRST_USER=$(jq -r '.data.users[0].username' "$TMP/users.json")
FIRST_PASS=$(jq -r '.data.accounts[0].password' "$TMP/users.json")
COURSE_SAVED=$(jq -r '.data.users[0].course // ""' "$TMP/users.json")
[ "$COURSE_SAVED" = "E2E Course" ] && ok "course persisted ($COURSE_SAVED)" || bad "course got '$COURSE_SAVED'"
echo "  first user: $FIRST_USER / $FIRST_PASS"

echo "[4] provision containers"
curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" -o "$TMP/prov.json" -w '%{http_code}' \
  -X POST "$BASE/platform/admin/containers/batch" -H 'Content-Type: application/json' \
  -d "{\"template_id\":\"$TPL_ID\",\"user_ids\":$USER_IDS,\"force\":false}" \
  | grep -q 200 && ok "provision accepted" || bad "provision: $(cat "$TMP/prov.json")"

echo "[5] wait for containers healthy"
RUNNING=0
for i in $(seq 1 60); do
  RUNNING=$(curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" "$BASE/platform/admin/containers" | jq '[.data[] | select(.status=="running")] | length')
  [ "$RUNNING" = "$COUNT" ] && break
  sleep 3
done
[ "$RUNNING" = "$COUNT" ] && ok "containers running ($RUNNING/$COUNT)" || bad "running=$RUNNING (need $COUNT)"

echo "[6] student login"
STU_LOGIN_CODE=$(curl -s -c "$STU_JAR" -o /dev/null -w '%{http_code}' \
  -X POST "$BASE/platform/auth/login" -H 'Content-Type: application/json' \
  -d "{\"username\":\"$FIRST_USER\",\"password\":\"$FIRST_PASS\"}")
[ "$STU_LOGIN_CODE" = "200" ] && ok "student login 200" || bad "student login (HTTP $STU_LOGIN_CODE)"

echo "[6.5] concurrent student access (simultaneous logins + proxy)"
CONC_FAIL=0
declare -a STU_JARS
# all students log in at the same time (their containers are already running)
for i in $(seq 0 $((COUNT-1))); do
  U=$(jq -r ".data.users[$i].username" "$TMP/users.json")
  P=$(jq -r ".data.accounts[$i].password" "$TMP/users.json")
  STU_JARS[$i]="$TMP/stu$i.cookies"
  curl -s -c "${STU_JARS[$i]}" -o /dev/null -w '%{http_code}' \
    -X POST "$BASE/platform/auth/login" -H 'Content-Type: application/json' \
    -d "{\"username\":\"$U\",\"password\":\"$P\"}" > "$TMP/conc_login_$i.code" &
done
wait
for i in $(seq 0 $((COUNT-1))); do
  [ "$(cat "$TMP/conc_login_$i.code")" = "200" ] || CONC_FAIL=$((CONC_FAIL+1))
done
# all logged in, now hit their own containers simultaneously
for i in $(seq 0 $((COUNT-1))); do
  curl -s -b "${STU_JARS[$i]}" -o "$TMP/conc_body_$i.html" -w '%{http_code}' \
    "$BASE/" > "$TMP/conc_proxy_$i.code" &
done
wait
for i in $(seq 0 $((COUNT-1))); do
  CODE=$(cat "$TMP/conc_proxy_$i.code")
  grep -qi "<!doctype html>" "$TMP/conc_body_$i.html" || CODE="BAD"
  [ "$CODE" = "200" ] || CONC_FAIL=$((CONC_FAIL+1))
done
if [ "$CONC_FAIL" -eq 0 ]; then
  ok "concurrent student login + proxy ($COUNT/$COUNT)"
else
  bad "concurrent access failures: $CONC_FAIL"
fi

echo "[7] proxy access root path -> container"
BODY=$(curl -s -b "$STU_JAR" -c "$STU_JAR" -w '\n%{http_code}' "$BASE/")
CODE=$(echo "$BODY" | tail -1)
echo "$BODY" | head -1 | grep -qi "<!doctype html>" && [ "$CODE" = "200" ] && ok "got HTML from container (code $CODE)" || bad "expected HTML from container, code=$CODE"

echo "[8] root without auth -> SPA (not container)"
BODY2=$(curl -s -w '\n%{http_code}' "$BASE/")
CODE2=$(echo "$BODY2" | tail -1)
echo "$BODY2" | grep -qi "<title>DevCapsule" && [ "$CODE2" = "200" ] && ok "anonymous gets platform SPA (code $CODE2)" || bad "expected SPA, code=$CODE2"

echo "[9] root as admin -> SPA (not container)"
ADMIN_ROOT=$(curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" -w '\n%{http_code}' "$BASE/")
CODE3=$(echo "$ADMIN_ROOT" | tail -1)
echo "$ADMIN_ROOT" | grep -qi "<title>DevCapsule" && [ "$CODE3" = "200" ] && ok "admin gets platform SPA (code $CODE3)" || bad "expected SPA, code=$CODE3"

echo "[10] batch action: stop all via users/batch/action"
curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" -o "$TMP/bstop.json" -w '%{http_code}' \
  -X POST "$BASE/platform/admin/users/batch/action" -H 'Content-Type: application/json' \
  -d "{\"user_ids\":$USER_IDS,\"action\":\"stop\"}" \
  | grep -q 200 && ok "batch stop accepted" || bad "batch stop"
OK_STOP=$(jq '[.data[] | select(.ok==true)] | length' "$TMP/bstop.json")
[ "$OK_STOP" = "$COUNT" ] && ok "batch stop ok=$OK_STOP" || bad "batch stop ok=$OK_STOP"
# grab a container record id from the admin list to test the single-start action
CID=$(curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" "$BASE/platform/admin/containers" | jq -r '.data[0].id // empty')
if [ -n "$CID" ]; then
  curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" -o /dev/null -w '%{http_code}' -X POST "$BASE/platform/admin/containers/$CID/start" | grep -q 200 && ok "start accepted" || bad "start"
  STATUS=""
  for i in $(seq 1 20); do
    STATUS=$(curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" "$BASE/platform/admin/containers" | jq -r --arg id "$CID" '.data[] | select(.id==$id) | .status // "unknown"')
    [ "$STATUS" = "running" ] && break
    sleep 2
  done
  [ "$STATUS" = "running" ] && ok "single start running" || bad "single start not running (status='${STATUS:-none}')"
else
  bad "no container id to test start"
fi

echo "[11] student self-service change password"
curl -s -b "$STU_JAR" -c "$STU_JAR" -o /dev/null -w '%{http_code}' -X POST "$BASE/platform/auth/change-password" -H 'Content-Type: application/json' \
  -d "{\"old_password\":\"wrong\",\"new_password\":\"newpass456\"}" \
  | grep -q 403 && ok "change password rejects wrong old password" || bad "change password wrong old"
curl -s -b "$STU_JAR" -c "$STU_JAR" -o /dev/null -w '%{http_code}' -X POST "$BASE/platform/auth/change-password" -H 'Content-Type: application/json' \
  -d "{\"old_password\":\"$FIRST_PASS\",\"new_password\":\"newpass456\"}" \
  | grep -q 200 && ok "change password 200" || bad "change password"
curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/platform/auth/login" -H 'Content-Type: application/json' \
  -d "{\"username\":\"$FIRST_USER\",\"password\":\"newpass456\"}" | grep -q 200 && ok "login with new password 200" || bad "login with new password"

echo "[12] dashboard stats"
curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" "$BASE/platform/admin/stats/dashboard" | jq -e '.data.users.total >= '"$COUNT"'' >/dev/null && ok "dashboard users>=COUNT" || bad "dashboard"
REQS=0
for i in $(seq 1 10); do
  REQS=$(curl -s -b "$ADMIN_JAR" -c "$ADMIN_JAR" "$BASE/platform/admin/stats/dashboard" | jq '.data.requests.count')
  [ "${REQS:-0}" -gt 0 ] && break
  sleep 1
done
[ "${REQS:-0}" -gt 0 ] && ok "requests counted ($REQS)" || bad "requests"

echo "[13] admin UI served"
curl -s "$BASE/admin" | grep -qi "<div id=\"app\">" && ok "SPA served" || bad "SPA"

echo
echo "E2E RESULT: $pass passed, $fail failed"
if [ "$fail" -eq 0 ]; then
  echo "ALL GREEN"
else
  echo "HAS FAILURES"
  exit 1
fi
