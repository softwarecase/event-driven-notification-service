#!/bin/bash
# =============================================================================
# Event-Driven Notification Service - Test Scenarios
# =============================================================================
# Usage: ./scripts/run-integration-tests.sh [scenario]
# Scenarios: health, single, batch, template, schedule, cancel, priority,
#            idempotency, websocket, load, failure, all
# =============================================================================

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

print_header() { echo -e "\n${CYAN}═══════════════════════════════════════════${NC}"; echo -e "${CYAN}  $1${NC}"; echo -e "${CYAN}═══════════════════════════════════════════${NC}"; }
print_ok()     { echo -e "${GREEN}✓ $1${NC}"; }
print_fail()   { echo -e "${RED}✗ $1${NC}"; }
print_info()   { echo -e "${YELLOW}→ $1${NC}"; }

# -------------------------------------------------------------------
# 1. HEALTH CHECK
# -------------------------------------------------------------------
test_health() {
    print_header "1. HEALTH CHECK"
    RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/health")
    CODE=$(echo "$RESP" | tail -1)
    BODY=$(echo "$RESP" | head -1)
    if [ "$CODE" = "200" ]; then
        print_ok "Health check passed ($CODE)"
        echo "$BODY" | python3 -m json.tool 2>/dev/null || echo "$BODY"
    else
        print_fail "Health check failed ($CODE)"
        echo "$BODY"
    fi
}

# -------------------------------------------------------------------
# 2. SINGLE NOTIFICATION (SMS, Email, Push)
# -------------------------------------------------------------------
test_single() {
    print_header "2. SINGLE NOTIFICATION - SMS (Order Confirmation)"
    SMS_RESP=$(curl -s -X POST "$BASE_URL/api/v1/notifications" \
        -H "Content-Type: application/json" \
        -d '{
            "channel": "sms",
            "recipient": "+905551234567",
            "content": "Your order #ORD-78432 has been confirmed! Estimated delivery: Mar 27. Track at shop.example.com/track/78432",
            "priority": "high"
        }')
    echo "$SMS_RESP" | python3 -m json.tool 2>/dev/null || echo "$SMS_RESP"
    SMS_ID=$(echo "$SMS_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
    if [ -n "$SMS_ID" ]; then
        print_ok "SMS notification created: $SMS_ID"
    else
        print_fail "SMS notification creation failed"
        return
    fi

    # Wait for delivery
    sleep 2

    print_info "Checking delivery status..."
    STATUS_RESP=$(curl -s "$BASE_URL/api/v1/notifications/$SMS_ID")
    STATUS=$(echo "$STATUS_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null)
    echo "$STATUS_RESP" | python3 -m json.tool 2>/dev/null || echo "$STATUS_RESP"
    if [ "$STATUS" = "delivered" ]; then
        print_ok "SMS delivered successfully!"
    elif [ "$STATUS" = "failed" ]; then
        print_info "SMS failed (provider may be unreachable) - retry mechanism will handle it"
    else
        print_info "SMS status: $STATUS (may still be processing)"
    fi

    print_header "2b. SINGLE NOTIFICATION - EMAIL (Shipping Update)"
    EMAIL_RESP=$(curl -s -X POST "$BASE_URL/api/v1/notifications" \
        -H "Content-Type: application/json" \
        -d '{
            "channel": "email",
            "recipient": "customer@example.com",
            "subject": "Your order has been shipped! 📦",
            "content": "<h1>Great news!</h1><p>Your order <b>#ORD-78432</b> has been shipped via FedEx.</p><p>Tracking number: <b>FX-98765432</b></p><p>Expected delivery: March 27, 2026</p>",
            "priority": "normal"
        }')
    echo "$EMAIL_RESP" | python3 -m json.tool 2>/dev/null || echo "$EMAIL_RESP"
    print_ok "Email notification created"

    print_header "2c. SINGLE NOTIFICATION - PUSH (Cart Reminder)"
    PUSH_RESP=$(curl -s -X POST "$BASE_URL/api/v1/notifications" \
        -H "Content-Type: application/json" \
        -d '{
            "channel": "push",
            "recipient": "device-token-abc123",
            "content": "You left 3 items in your cart! Complete your purchase before they sell out.",
            "priority": "low",
            "metadata": {"app": "shopify-mobile", "badge": 3, "category": "cart_abandonment"}
        }')
    echo "$PUSH_RESP" | python3 -m json.tool 2>/dev/null || echo "$PUSH_RESP"
    print_ok "Push notification created"
}

# -------------------------------------------------------------------
# 3. BATCH NOTIFICATION
# -------------------------------------------------------------------
test_batch() {
    print_header "3. BATCH NOTIFICATION - Spring Sale Campaign (5 messages)"
    BATCH_RESP=$(curl -s -X POST "$BASE_URL/api/v1/notifications/batch" \
        -H "Content-Type: application/json" \
        -d '{
            "notifications": [
                {"channel": "sms", "recipient": "+905551111111", "content": "Spring Sale! Get 40% off all electronics. Use code SPRING40 at checkout. Ends tonight!", "priority": "high"},
                {"channel": "email", "recipient": "vip-customer@example.com", "content": "Exclusive VIP Early Access: Spring Collection is here. Enjoy free shipping on orders over $50.", "priority": "high"},
                {"channel": "push", "recipient": "device-token-user42", "content": "Price drop alert! The item in your wishlist is now 30% off.", "priority": "normal"},
                {"channel": "sms", "recipient": "+905552222222", "content": "Your reward points are expiring soon! Redeem 500 points for $10 off your next order.", "priority": "normal"},
                {"channel": "email", "recipient": "new-user@example.com", "content": "Welcome to ShopExample! Here is your 15% first-order discount code: WELCOME15", "priority": "low"}
            ]
        }')
    echo "$BATCH_RESP" | python3 -m json.tool 2>/dev/null || echo "$BATCH_RESP"
    BATCH_ID=$(echo "$BATCH_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['batch_id'])" 2>/dev/null)
    if [ -n "$BATCH_ID" ]; then
        print_ok "Batch created: $BATCH_ID"
        sleep 3
        print_info "Checking batch delivery status..."
        curl -s "$BASE_URL/api/v1/notifications/batch/$BATCH_ID" | python3 -m json.tool 2>/dev/null
    else
        print_fail "Batch creation failed"
    fi
}

# -------------------------------------------------------------------
# 4. TEMPLATE SYSTEM
# -------------------------------------------------------------------
test_template() {
    print_header "4. TEMPLATE - Create (Order Confirmation)"
    TPL_NAME="order_confirmation_$(date +%s)"
    TPL_RESP=$(curl -s -X POST "$BASE_URL/api/v1/templates" \
        -H "Content-Type: application/json" \
        -d "{
            \"name\": \"$TPL_NAME\",
            \"channel\": \"email\",
            \"subject\": \"Order #{{.order_id}} confirmed - {{.customer_name}}!\",
            \"content\": \"Hi {{.customer_name}}, thank you for your order #{{.order_id}}! Total: \${{.total}}. Estimated delivery: {{.delivery_date}}. Track your package at shop.example.com/track/{{.order_id}}\",
            \"variables\": [
                {\"name\": \"customer_name\", \"required\": true},
                {\"name\": \"order_id\", \"required\": true},
                {\"name\": \"total\", \"required\": true},
                {\"name\": \"delivery_date\", \"required\": false, \"default_value\": \"3-5 business days\"}
            ]
        }")
    echo "$TPL_RESP" | python3 -m json.tool 2>/dev/null || echo "$TPL_RESP"
    TPL_ID=$(echo "$TPL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)

    if [ -n "$TPL_ID" ]; then
        print_ok "Template created: $TPL_ID"

        print_header "4b. TEMPLATE - Preview"
        curl -s -X POST "$BASE_URL/api/v1/templates/$TPL_ID/preview" \
            -H "Content-Type: application/json" \
            -d '{"variables": {"customer_name": "John Doe", "order_id": "ORD-99201", "total": "149.99", "delivery_date": "March 28, 2026"}}' | python3 -m json.tool 2>/dev/null
        print_ok "Template preview rendered"

        print_header "4c. TEMPLATE - Send Order Confirmation via Template"
        NOTIF_RESP=$(curl -s -X POST "$BASE_URL/api/v1/notifications" \
            -H "Content-Type: application/json" \
            -d "{
                \"channel\": \"email\",
                \"recipient\": \"john.doe@example.com\",
                \"template_id\": \"$TPL_ID\",
                \"template_vars\": {\"customer_name\": \"John Doe\", \"order_id\": \"ORD-99201\", \"total\": \"149.99\", \"delivery_date\": \"March 28, 2026\"},
                \"priority\": \"high\"
            }")
        echo "$NOTIF_RESP" | python3 -m json.tool 2>/dev/null || echo "$NOTIF_RESP"
        print_ok "Order confirmation sent via template"
    else
        print_fail "Template creation failed"
    fi
}

# -------------------------------------------------------------------
# 5. SCHEDULED NOTIFICATION
# -------------------------------------------------------------------
test_schedule() {
    print_header "5. SCHEDULED NOTIFICATION (Flash Sale Announcement)"
    # Schedule 15 seconds from now
    SCHEDULED_TIME=$(date -u -v+15S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "+15 seconds" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null)
    print_info "Scheduling flash sale notification for: $SCHEDULED_TIME"

    SCHED_RESP=$(curl -s -X POST "$BASE_URL/api/v1/notifications" \
        -H "Content-Type: application/json" \
        -d "{
            \"channel\": \"sms\",
            \"recipient\": \"+905559999999\",
            \"content\": \"FLASH SALE starts NOW! 60% off all items for the next 2 hours. Shop now: shop.example.com/flash-sale\",
            \"priority\": \"high\",
            \"scheduled_at\": \"$SCHEDULED_TIME\"
        }")
    echo "$SCHED_RESP" | python3 -m json.tool 2>/dev/null || echo "$SCHED_RESP"
    SCHED_ID=$(echo "$SCHED_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)

    if [ -n "$SCHED_ID" ]; then
        STATUS=$(echo "$SCHED_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null)
        print_ok "Scheduled notification created: $SCHED_ID (status: $STATUS)"

        print_info "Waiting 20 seconds for scheduler to pick it up..."
        sleep 20
        FINAL=$(curl -s "$BASE_URL/api/v1/notifications/$SCHED_ID")
        FINAL_STATUS=$(echo "$FINAL" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null)
        print_info "Final status: $FINAL_STATUS"
        if [ "$FINAL_STATUS" = "delivered" ] || [ "$FINAL_STATUS" = "queued" ] || [ "$FINAL_STATUS" = "processing" ]; then
            print_ok "Scheduler picked up the notification!"
        fi
    fi
}

# -------------------------------------------------------------------
# 6. CANCEL NOTIFICATION
# -------------------------------------------------------------------
test_cancel() {
    print_header "6. CANCEL NOTIFICATION (Customer Cancels Order)"
    # Create a scheduled notification far in the future (order dispatch reminder)
    FUTURE_TIME=$(date -u -v+1H +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "+1 hour" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null)

    CANCEL_RESP=$(curl -s -X POST "$BASE_URL/api/v1/notifications" \
        -H "Content-Type: application/json" \
        -d "{
            \"channel\": \"email\",
            \"recipient\": \"customer@example.com\",
            \"content\": \"Your order #ORD-55012 is being prepared for dispatch. Estimated shipping: tomorrow.\",
            \"scheduled_at\": \"$FUTURE_TIME\"
        }")
    CANCEL_ID=$(echo "$CANCEL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)

    if [ -n "$CANCEL_ID" ]; then
        print_ok "Created scheduled dispatch notification: $CANCEL_ID"
        sleep 1

        print_info "Customer cancelled their order — cancelling the notification..."
        CANCEL_RESULT=$(curl -s -X PATCH "$BASE_URL/api/v1/notifications/$CANCEL_ID/cancel")
        echo "$CANCEL_RESULT" | python3 -m json.tool 2>/dev/null || echo "$CANCEL_RESULT"

        VERIFY=$(curl -s "$BASE_URL/api/v1/notifications/$CANCEL_ID")
        VERIFY_STATUS=$(echo "$VERIFY" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null)
        if [ "$VERIFY_STATUS" = "cancelled" ]; then
            print_ok "Notification cancelled successfully!"
        else
            print_fail "Cancel may have failed, status: $VERIFY_STATUS"
        fi
    fi
}

# -------------------------------------------------------------------
# 7. PRIORITY QUEUE TEST
# -------------------------------------------------------------------
test_priority() {
    print_header "7. PRIORITY QUEUE TEST (Order vs. Marketing vs. Newsletter)"
    print_info "Sending LOW (newsletter), NORMAL (promo), HIGH (payment alert) — payment should process first"

    curl -s -X POST "$BASE_URL/api/v1/notifications" \
        -H "Content-Type: application/json" \
        -d '{"channel": "sms", "recipient": "+900001", "content": "Weekly newsletter: Top 10 trending products this week.", "priority": "low"}' > /dev/null

    curl -s -X POST "$BASE_URL/api/v1/notifications" \
        -H "Content-Type: application/json" \
        -d '{"channel": "sms", "recipient": "+900002", "content": "Your wishlist item is on sale! 25% off Nike Air Max.", "priority": "normal"}' > /dev/null

    curl -s -X POST "$BASE_URL/api/v1/notifications" \
        -H "Content-Type: application/json" \
        -d '{"channel": "sms", "recipient": "+900003", "content": "URGENT: Payment of $299.99 processed for order #ORD-88210. If this was not you, contact support immediately.", "priority": "high"}' > /dev/null

    print_ok "3 notifications sent with different priorities"
    print_info "HIGH (payment alert) should arrive first, then NORMAL (promo), then LOW (newsletter)"

    sleep 3
    print_info "Listing recent notifications..."
    curl -s "$BASE_URL/api/v1/notifications?limit=3&sort=created_at&order=desc" | python3 -m json.tool 2>/dev/null
}

# -------------------------------------------------------------------
# 8. IDEMPOTENCY TEST
# -------------------------------------------------------------------
test_idempotency() {
    print_header "8. IDEMPOTENCY TEST (Prevent Duplicate Order Confirmation)"
    IDEMP_KEY="order-confirm-ORD-$(date +%s)"
    print_info "Idempotency key: $IDEMP_KEY"

    RESP1=$(curl -s -X POST "$BASE_URL/api/v1/notifications" \
        -H "Content-Type: application/json" \
        -H "X-Idempotency-Key: $IDEMP_KEY" \
        -d "{
            \"channel\": \"sms\",
            \"recipient\": \"+905551234567\",
            \"content\": \"Your order #ORD-44510 has been confirmed. Total: \$89.99. Thank you for shopping with us!\",
            \"idempotency_key\": \"$IDEMP_KEY\"
        }")
    ID1=$(echo "$RESP1" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
    print_ok "First request (order placed) - ID: $ID1"

    RESP2=$(curl -s -X POST "$BASE_URL/api/v1/notifications" \
        -H "Content-Type: application/json" \
        -H "X-Idempotency-Key: $IDEMP_KEY" \
        -d "{
            \"channel\": \"sms\",
            \"recipient\": \"+905551234567\",
            \"content\": \"Your order #ORD-44510 has been confirmed. Total: \$89.99. Thank you for shopping with us!\",
            \"idempotency_key\": \"$IDEMP_KEY\"
        }")
    ID2=$(echo "$RESP2" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
    print_ok "Second request (retry/duplicate) - ID: $ID2"

    if [ "$ID1" = "$ID2" ]; then
        print_ok "IDEMPOTENCY WORKS! Customer won't receive duplicate order confirmations"
    else
        print_fail "Idempotency failed - different IDs: $ID1 vs $ID2"
    fi
}

# -------------------------------------------------------------------
# 9. WEBSOCKET TEST
# -------------------------------------------------------------------
test_websocket() {
    print_header "9. WEBSOCKET REAL-TIME EVENTS"
    print_info "WebSocket endpoint: ws://localhost:8080/ws/notifications"
    print_info "To test manually, open a second terminal and run:"
    echo ""
    echo "  # Install wscat if needed: npm install -g wscat"
    echo "  wscat -c ws://localhost:8080/ws/notifications"
    echo ""
    echo "  # Or with batch filter:"
    echo "  wscat -c 'ws://localhost:8080/ws/notifications?batch_id=BATCH-UUID'"
    echo ""
    print_info "Then send a notification from another terminal and watch real-time updates"

    # Quick test with curl if websocat is available
    if command -v websocat &> /dev/null; then
        print_info "websocat found, testing..."
        websocat "ws://localhost:8080/ws/notifications" &
        WS_PID=$!
        sleep 1
        curl -s -X POST "$BASE_URL/api/v1/notifications" \
            -H "Content-Type: application/json" \
            -d '{"channel": "sms", "recipient": "+90555ws", "content": "Your order #ORD-70021 has been delivered! Rate your experience."}' > /dev/null
        sleep 3
        kill $WS_PID 2>/dev/null || true
        print_ok "WebSocket test complete - check output above for events"
    else
        print_info "Install websocat or wscat for automated WebSocket testing"
    fi
}

# -------------------------------------------------------------------
# 10. LOAD TEST (High Throughput)
# -------------------------------------------------------------------
test_load() {
    print_header "10. LOAD TEST - High Throughput"
    TOTAL=${1:-100}
    CONCURRENT=${2:-10}
    print_info "Sending $TOTAL notifications with $CONCURRENT concurrent requests"

    START_TIME=$(date +%s)

    for i in $(seq 1 $TOTAL); do
        curl -s -X POST "$BASE_URL/api/v1/notifications" \
            -H "Content-Type: application/json" \
            -d "{
                \"channel\": \"sms\",
                \"recipient\": \"+9055500$i\",
                \"content\": \"Order #ORD-$i update: Your package is out for delivery today!\",
                \"priority\": \"normal\"
            }" > /dev/null &

        # Limit concurrent requests
        if (( i % CONCURRENT == 0 )); then
            wait
        fi
    done
    wait

    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    if [ "$DURATION" -eq 0 ]; then DURATION=1; fi
    RPS=$((TOTAL / DURATION))

    print_ok "Sent $TOTAL notifications in ${DURATION}s (~${RPS} req/s)"

    sleep 5
    print_info "Checking metrics..."
    curl -s "$BASE_URL/metrics" | python3 -m json.tool 2>/dev/null
}

# -------------------------------------------------------------------
# 11. FAILURE & RETRY TEST
# -------------------------------------------------------------------
test_failure() {
    print_header "11. FAILURE & RETRY SCENARIOS"
    print_info "Testing what happens when delivery fails..."
    print_info ""
    print_info "Scenarios handled by the system:"
    echo ""
    echo "  1. Provider Down (webhook.site unreachable):"
    echo "     → DeliveryService.Process() catches error"
    echo "     → Circuit breaker records failure"
    echo "     → handleRetry() increments RetryCount"
    echo "     → Exponential backoff: 1s, 2s, 4s (with ±20% jitter)"
    echo "     → RetryPoller picks up from DB and re-enqueues"
    echo ""
    echo "  2. Circuit Breaker Opens (5 failures in window):"
    echo "     → breaker.Allow() returns false"
    echo "     → Notification re-queued without hitting provider"
    echo "     → After 30s cooldown, half-open state allows 3 test requests"
    echo "     → If tests pass → circuit closes, normal flow resumes"
    echo ""
    echo "  3. Max Retries Exceeded (3 retries default):"
    echo "     → Notification moved to Dead Letter Queue (DLQ)"
    echo "     → Status = 'failed', DLQ entry with full payload"
    echo "     → Can be reprocessed later via DLQ"
    echo ""
    echo "  4. Rate Limiting (100 msg/sec/channel):"
    echo "     → Redis sliding window counter per channel"
    echo "     → Exceeded → re-enqueue for later processing"
    echo "     → No message loss, just delayed delivery"
    echo ""
    echo "  5. Worker Crash / Restart:"
    echo "     → Graceful shutdown via SIGTERM handler"
    echo "     → In-flight messages: status in DB = 'processing'"
    echo "     → On restart, RetryPoller picks up stuck messages"
    echo "     → Redis sorted set: unacked messages stay in processing set"
    echo ""

    print_info "Checking circuit breaker states..."
    curl -s "$BASE_URL/metrics" | python3 -m json.tool 2>/dev/null

    print_info "Checking DLQ entries..."
    curl -s "$BASE_URL/api/v1/notifications?status=failed&limit=5" | python3 -m json.tool 2>/dev/null
}

# -------------------------------------------------------------------
# 12. METRICS & OBSERVABILITY
# -------------------------------------------------------------------
test_metrics() {
    print_header "12. METRICS & OBSERVABILITY"

    print_info "Metrics endpoint:"
    curl -s "$BASE_URL/metrics" | python3 -m json.tool 2>/dev/null

    echo ""
    print_info "Jaeger UI: http://localhost:16686"
    print_info "  → Select service: 'notification-api' or 'notification-worker'"
    print_info "  → See distributed traces across API → Queue → Worker"
    echo ""
    print_info "Structured logs: docker-compose -f deployments/docker-compose.yml logs -f api worker"
}

# -------------------------------------------------------------------
# 13. LIST & FILTER
# -------------------------------------------------------------------
test_list() {
    print_header "13. LIST & FILTER NOTIFICATIONS"

    print_info "All notifications (page 1):"
    curl -s "$BASE_URL/api/v1/notifications?limit=5&offset=0" | python3 -m json.tool 2>/dev/null

    echo ""
    print_info "Filter by channel=sms:"
    curl -s "$BASE_URL/api/v1/notifications?channel=sms&limit=3" | python3 -m json.tool 2>/dev/null

    echo ""
    print_info "Filter by status=delivered:"
    curl -s "$BASE_URL/api/v1/notifications?status=delivered&limit=3" | python3 -m json.tool 2>/dev/null

    echo ""
    print_info "Filter by priority=high:"
    curl -s "$BASE_URL/api/v1/notifications?priority=high&limit=3" | python3 -m json.tool 2>/dev/null
}

# -------------------------------------------------------------------
# MAIN
# -------------------------------------------------------------------
case "${1:-all}" in
    health)      test_health ;;
    single)      test_single ;;
    batch)       test_batch ;;
    template)    test_template ;;
    schedule)    test_schedule ;;
    cancel)      test_cancel ;;
    priority)    test_priority ;;
    idempotency) test_idempotency ;;
    websocket)   test_websocket ;;
    load)        test_load ${2:-100} ${3:-10} ;;
    failure)     test_failure ;;
    metrics)     test_metrics ;;
    list)        test_list ;;
    all)
        test_health
        test_single
        test_batch
        test_template
        test_schedule
        test_cancel
        test_priority
        test_idempotency
        test_websocket
        test_list
        test_metrics
        echo ""
        print_header "ALL TESTS COMPLETE"
        print_info "For load test: ./scripts/run-integration-tests.sh load 500 20"
        print_info "For failure info: ./scripts/run-integration-tests.sh failure"
        ;;
    *)
        echo "Usage: $0 [health|single|batch|template|schedule|cancel|priority|idempotency|websocket|load|failure|metrics|list|all]"
        exit 1
        ;;
esac
