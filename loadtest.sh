#!/bin/bash
# Load Test: Webhook Delivery System
# Tests fan-out throughput with many subscribers

API="http://localhost:8080/api/v1"
MOCK="http://localhost:9090"
NUM_SUBSCRIBERS=100
NUM_EVENTS=10

ts() { python3 -c 'import time; print(int(time.time()*1000))'; }

echo "=== Webhook Delivery System — Load Test ==="
echo ""

# Step 1: Create subscribers
echo "Creating $NUM_SUBSCRIBERS subscribers..."
start_create=$(ts)

for i in $(seq 1 $NUM_SUBSCRIBERS); do
  curl -s -X POST "$API/subscribers" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"Load Test Sub $i\",
      \"endpoint_url\": \"$MOCK/webhook/success\",
      \"event_types\": [\"load.test\"],
      \"rate_limit_per_second\": 0
    }" > /dev/null &
done
wait

end_create=$(ts)
create_ms=$((end_create - start_create))
echo "  Created $NUM_SUBSCRIBERS subscribers in ${create_ms}ms"
echo ""

# Step 2: Verify subscriber count
sub_count=$(curl -s "$API/subscribers" | python3 -c "import sys,json; data=json.load(sys.stdin); print(len(data))" 2>/dev/null)
echo "  Active subscribers: $sub_count"
echo ""

# Step 3: Fire events and measure fan-out
echo "Firing $NUM_EVENTS events (each fans out to $NUM_SUBSCRIBERS subscribers)..."
total_deliveries=$((NUM_SUBSCRIBERS * NUM_EVENTS))
echo "  Expected total deliveries: $total_deliveries"
echo ""

start_fire=$(ts)

for i in $(seq 1 $NUM_EVENTS); do
  curl -s -X POST "$API/events" \
    -H "Content-Type: application/json" \
    -d "{
      \"event_type\": \"load.test\",
      \"payload\": {\"test_id\": $i, \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}
    }" > /dev/null &
done
wait

end_fire=$(ts)
fire_ms=$((end_fire - start_fire))
echo "  Events queued in ${fire_ms}ms"
echo ""

# Step 4: Wait for deliveries to complete
echo "Waiting for deliveries to complete..."
max_wait=60
elapsed=0

while [ $elapsed -lt $max_wait ]; do
  sleep 2
  elapsed=$((elapsed + 2))

  metrics=$(curl -s "$API/metrics" 2>/dev/null)
  delivered=$(echo "$metrics" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total_deliveries', 0))" 2>/dev/null)
  success_rate=$(echo "$metrics" | python3 -c "import sys,json; d=json.load(sys.stdin); print(round(d.get('success_rate', 0), 1))" 2>/dev/null)
  queue=$(echo "$metrics" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('queue_depth', 0))" 2>/dev/null)

  printf "  [%2ds] Delivered: %s / %s | Success rate: %s%% | Queue: %s\n" "$elapsed" "$delivered" "$total_deliveries" "$success_rate" "$queue"

  if [ "$queue" = "0" ] && [ "$delivered" != "0" ] && [ "$delivered" -ge "$total_deliveries" ] 2>/dev/null; then
    break
  fi
done

end_total=$(ts)
total_ms=$((end_total - start_fire))
total_secs=$(python3 -c "print(round($total_ms / 1000, 2))")
throughput=$(python3 -c "print(round($total_deliveries / ($total_ms / 1000), 1))" 2>/dev/null)

echo ""
echo "=== Results ==="
echo ""

# Final metrics
metrics=$(curl -s "$API/metrics" 2>/dev/null)
echo "$metrics" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(f'  Total deliveries:    {d.get(\"total_deliveries\", 0)}')
print(f'  Success rate:        {round(d.get(\"success_rate\", 0), 1)}%')
print(f'  Avg response time:   {d.get(\"avg_response_time_ms\", 0)}ms')
print(f'  Dead letters:        {d.get(\"dead_letter_count\", 0)}')
"

echo "  Total time:          ${total_secs}s"
echo "  Throughput:          ~${throughput} deliveries/sec"
echo ""
echo "=== Load Test Complete ==="
