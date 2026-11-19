#!/bin/sh
# End-to-end smoke test against a running stack (make up).
# Creates one notification per audience type and waits until each one has
# been fully produced and delivered (or is only waiting for scheduled retries).
set -eu
API="${API:-http://localhost:8080/api/v1}"
fail() { echo "FAIL: $*" >&2; exit 1; }

post() { curl -sf -X POST "$API/$1" -H 'Content-Type: application/json' -d "$2"; }
field() { sed -n "s/.*\"$1\":\"\([^\"]*\)\".*/\1/p" | head -n1; }

echo "== readiness"
curl -sf "${API%/api/v1}/ready" >/dev/null || fail "panel not ready"

echo "== create notifications"
SQL_ID=$(post notifications '{"channel":"sms","priority":"high","audience":{"type":"SQL","sql":"SELECT id FROM master_contact WHERE city = '"'"'Bursa'"'"' AND loyalty_tier = '"'"'gold'"'"'"},"content":{"body":"e2e sql"}}' | field id)
FILTER_ID=$(post notifications '{"channel":"email","audience":{"type":"FILTER","filter":{"conditions":[{"field":"city","operator":"eq","value":"Ankara"},{"field":"loyalty_tier","operator":"eq","value":"silver"}]}},"content":{"subject":"Hi {{first_name}}","body":"<p>{{first_name}} {{last_name}}</p>","is_html":true},"customized_content":true}' | field id)
GROUP_ID=$(post notifications '{"channel":"push","priority":"low","audience":{"type":"GROUP","group_id":1},"content_id":1}' | field id)
DIRECT_ID=$(post notifications '{"channel":"sms","audience":{"type":"DIRECT","recipients":["+905551230001","+905551230002"]},"content":{"body":"e2e direct"}}' | field id)

echo "== idempotency"
K="e2e-$(date +%s)"
A=$(post notifications '{"channel":"sms","idempotency_key":"'"$K"'","audience":{"type":"DIRECT","recipients":["+905551230003"]},"content":{"body":"idem"}}' | field id)
B=$(post notifications '{"channel":"sms","idempotency_key":"'"$K"'","audience":{"type":"DIRECT","recipients":["+905551230003"]},"content":{"body":"idem"}}' | field id)
[ "$A" = "$B" ] || fail "idempotency key created two notifications"

echo "== validation"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/notifications" -H 'Content-Type: application/json' \
  -d '{"channel":"sms","audience":{"type":"SQL","sql":"DELETE FROM master_contact"},"content":{"body":"x"}}')
[ "$CODE" = "422" ] || fail "malicious SQL was not rejected (got $CODE)"

echo "== wait for processing"
for id in $SQL_ID $FILTER_ID $GROUP_ID $DIRECT_ID $A; do
  i=0
  while :; do
    BODY=$(curl -sf "$API/notifications/$id")
    STATUS=$(echo "$BODY" | field status)
    PENDING=$(echo "$BODY" | sed -n 's/.*"sending":\([0-9]*\).*/\1/p')
    if [ "$STATUS" = "COMPLETE" ] || { [ "$STATUS" = "SENDING" ] && [ "$PENDING" = "0" ]; }; then
      echo "   $id -> $STATUS"
      break
    fi
    [ "$STATUS" = "ERROR" ] && fail "$id failed: $BODY"
    i=$((i + 1)); [ $i -gt 60 ] && fail "$id stuck in $STATUS"
    sleep 1
  done
done
echo "OK"
