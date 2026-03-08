#!/usr/bin/env bash
# messaging_functionality_challenge.sh - Validates Messaging module core functionality and structure
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MODULE_NAME="Messaging"

PASS=0
FAIL=0
TOTAL=0

pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

echo "=== ${MODULE_NAME} Functionality Challenge ==="
echo ""

# Test 1: Required packages exist
echo "Test: Required packages exist"
pkgs_ok=true
for pkg in broker kafka rabbitmq producer consumer; do
    if [ ! -d "${MODULE_DIR}/pkg/${pkg}" ]; then
        fail "Missing package: pkg/${pkg}"
        pkgs_ok=false
    fi
done
if [ "$pkgs_ok" = true ]; then
    pass "All required packages present (broker, kafka, rabbitmq, producer, consumer)"
fi

# Test 2: MessageBroker interface is defined
echo "Test: MessageBroker interface is defined"
if grep -rq "type MessageBroker interface" "${MODULE_DIR}/pkg/broker/"; then
    pass "MessageBroker interface is defined in pkg/broker"
else
    fail "MessageBroker interface not found in pkg/broker"
fi

# Test 3: Message struct is defined
echo "Test: Message struct is defined"
if grep -rq "type Message struct" "${MODULE_DIR}/pkg/broker/"; then
    pass "Message struct is defined in pkg/broker"
else
    fail "Message struct not found in pkg/broker"
fi

# Test 4: Kafka implementation exists
echo "Test: Kafka implementation exists"
if grep -rq "type\s\+\w\+\s\+struct" "${MODULE_DIR}/pkg/kafka/"; then
    pass "Kafka implementation exists in pkg/kafka"
else
    fail "No Kafka implementation found in pkg/kafka"
fi

# Test 5: RabbitMQ implementation exists
echo "Test: RabbitMQ implementation exists"
if grep -rq "type\s\+\w\+\s\+struct" "${MODULE_DIR}/pkg/rabbitmq/"; then
    pass "RabbitMQ implementation exists in pkg/rabbitmq"
else
    fail "No RabbitMQ implementation found in pkg/rabbitmq"
fi

# Test 6: Subscription interface exists
echo "Test: Subscription interface exists"
if grep -rq "type Subscription interface" "${MODULE_DIR}/pkg/broker/"; then
    pass "Subscription interface is defined"
else
    fail "Subscription interface not found"
fi

# Test 7: Dead letter queue support
echo "Test: Dead letter queue support exists"
if grep -riq "dead.*letter\|dlq\|DeadLetter" "${MODULE_DIR}/pkg/"; then
    pass "Dead letter queue support found"
else
    fail "No dead letter queue support found"
fi

# Test 8: Producer functionality
echo "Test: Producer package has publish/send capability"
if grep -rq "Publish\|Send\|Produce" "${MODULE_DIR}/pkg/producer/"; then
    pass "Producer publish/send capability found"
else
    fail "No publish/send capability in producer"
fi

# Test 9: Consumer functionality
echo "Test: Consumer package has subscribe/consume capability"
if grep -rq "Subscribe\|Consume\|Receive" "${MODULE_DIR}/pkg/consumer/"; then
    pass "Consumer subscribe/consume capability found"
else
    fail "No subscribe/consume capability in consumer"
fi

# Test 10: Error handling types exist
echo "Test: Error handling types exist"
if grep -rq "BrokerError\|Error\|ErrorCode" "${MODULE_DIR}/pkg/broker/"; then
    pass "Error handling types found"
else
    fail "No error handling types found"
fi

echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="
[ "${FAIL}" -eq 0 ] && exit 0 || exit 1
