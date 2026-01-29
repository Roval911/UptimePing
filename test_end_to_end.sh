#!/bin/bash

echo "🎯 End-to-End Functional Tests for UptimePingPlatform"
echo "====================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test results
PASSED=0
FAILED=0

# Function to run test
run_test() {
    local test_name="$1"
    local command="$2"
    
    echo -e "\n📋 Testing: $test_name"
    
    if eval "$command" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ PASSED${NC}: $test_name"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}❌ FAILED${NC}: $test_name"
        ((FAILED++))
        return 1
    fi
}

# Function to test with output
test_with_output() {
    local test_name="$1"
    local command="$2"
    
    echo -e "\n📋 Testing: $test_name"
    echo "Output:"
    
    if eval "$command"; then
        echo -e "${GREEN}✅ PASSED${NC}: $test_name"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}❌ FAILED${NC}: $test_name"
        ((FAILED++))
        return 1
    fi
}

echo -e "\n${BLUE}🔧 Step 1: Create Test User via Auth Service${NC}"
echo "=============================================="

# Register a test user
REGISTER_RESPONSE=$(docker exec uptimeping-api-gateway wget --timeout=5 -qO- --post-data='{"email":"test@example.com","password":"testpassword123","tenant_name":"TestTenant"}' --header='Content-Type: application/json' http://localhost:8080/api/v1/auth/register 2>/dev/null)

if [ $? -eq 0 ] && [ -n "$REGISTER_RESPONSE" ]; then
    echo -e "${GREEN}✅ PASSED${NC}: User Registration"
    ((PASSED++))
    echo "Response: $REGISTER_RESPONSE"
    
    # Extract tokens from response
    ACCESS_TOKEN=$(echo "$REGISTER_RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    REFRESH_TOKEN=$(echo "$REGISTER_RESPONSE" | grep -o '"refresh_token":"[^"]*"' | cut -d'"' -f4)
    
    if [ -n "$ACCESS_TOKEN" ]; then
        echo -e "${GREEN}✅ PASSED${NC}: Access Token Extracted"
        ((PASSED++))
    else
        echo -e "${RED}❌ FAILED${NC}: Access Token Extraction"
        ((FAILED++))
    fi
else
    echo -e "${RED}❌ FAILED${NC}: User Registration"
    ((FAILED++))
fi

echo -e "\n${BLUE}🔧 Step 2: Test User Login${NC}"
echo "========================"

# Login with the test user
LOGIN_RESPONSE=$(docker exec uptimeping-api-gateway wget --timeout=5 -qO- --post-data='{"email":"test@example.com","password":"testpassword123"}' --header='Content-Type: application/json' http://localhost:8080/api/v1/auth/login 2>/dev/null)

if [ $? -eq 0 ] && [ -n "$LOGIN_RESPONSE" ]; then
    echo -e "${GREEN}✅ PASSED${NC}: User Login"
    ((PASSED++))
    echo "Response: $LOGIN_RESPONSE"
    
    # Extract tokens from login response
    ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    REFRESH_TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"refresh_token":"[^"]*"' | cut -d'"' -f4)
    
    if [ -n "$ACCESS_TOKEN" ]; then
        echo -e "${GREEN}✅ PASSED${NC}: Login Access Token Extracted"
        ((PASSED++))
        echo "Access Token: ${ACCESS_TOKEN:0:50}..."
    else
        echo -e "${RED}❌ FAILED${NC}: Login Access Token Extraction"
        ((FAILED++))
    fi
else
    echo -e "${RED}❌ FAILED${NC}: User Login"
    ((FAILED++))
fi

echo -e "\n${BLUE}🔧 Step 3: Create API Key${NC}"
echo "====================="

# Create API key
API_KEY_RESPONSE=$(docker exec uptimeping-api-gateway wget --timeout=5 -qO- --post-data='{"tenant_id":"test-tenant","name":"Test API Key"}' --header='Content-Type: application/json' http://localhost:8080/api/v1/auth/api-keys 2>/dev/null)

if [ $? -eq 0 ] && [ -n "$API_KEY_RESPONSE" ]; then
    echo -e "${GREEN}✅ PASSED${NC}: API Key Creation"
    ((PASSED++))
    echo "Response: $API_KEY_RESPONSE"
    
    # Extract API key
    API_KEY=$(echo "$API_KEY_RESPONSE" | grep -o '"key":"[^"]*"' | cut -d'"' -f4)
    API_SECRET=$(echo "$API_KEY_RESPONSE" | grep -o '"secret":"[^"]*"' | cut -d'"' -f4)
    
    if [ -n "$API_KEY" ] && [ -n "$API_SECRET" ]; then
        echo -e "${GREEN}✅ PASSED${NC}: API Key Extracted"
        ((PASSED++))
        echo "API Key: ${API_KEY:0:20}..."
    else
        echo -e "${RED}❌ FAILED${NC}: API Key Extraction"
        ((FAILED++))
    fi
else
    echo -e "${RED}❌ FAILED${NC}: API Key Creation"
    ((FAILED++))
fi

echo -e "\n${BLUE}🔧 Step 4: Create Check via Scheduler Service${NC}"
echo "=============================================="

# Create a test check
CHECK_RESPONSE=$(docker exec uptimeping-api-gateway wget --timeout=5 -qO- --post-data='{"name":"Test Check","url":"https://httpbin.org/status/200","check_type":"http","interval":60,"timeout":30}' --header='Content-Type: application/json' http://localhost:8080/api/v1/scheduler/checks 2>/dev/null)

if [ $? -eq 0 ] && [ -n "$CHECK_RESPONSE" ]; then
    echo -e "${GREEN}✅ PASSED${NC}: Check Creation"
    ((PASSED++))
    echo "Response: $CHECK_RESPONSE"
    
    # Extract check ID
    CHECK_ID=$(echo "$CHECK_RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    
    if [ -n "$CHECK_ID" ]; then
        echo -e "${GREEN}✅ PASSED${NC}: Check ID Extracted"
        ((PASSED++))
        echo "Check ID: $CHECK_ID"
    else
        echo -e "${RED}❌ FAILED${NC}: Check ID Extraction"
        ((FAILED++))
    fi
else
    echo -e "${RED}❌ FAILED${NC}: Check Creation"
    ((FAILED++))
fi

echo -e "\n${BLUE}🔧 Step 5: Test Core Service Directly${NC}"
echo "=================================="

# Test Core Service health
CORE_HEALTH=$(docker exec uptimeping-core-service wget --timeout=5 -qO- http://localhost:50054/health 2>/dev/null)

if [ $? -eq 0 ] && [ -n "$CORE_HEALTH" ]; then
    echo -e "${GREEN}✅ PASSED${NC}: Core Service Health"
    ((PASSED++))
    echo "Response: $CORE_HEALTH"
else
    echo -e "${RED}❌ FAILED${NC}: Core Service Health"
    ((FAILED++))
fi

echo -e "\n${BLUE}🔧 Step 6: Test Database Operations${NC}"
echo "================================="

# Test database connectivity
DB_TEST=$(docker exec uptimeping-postgres psql -U uptimeping -d uptimeping -c "SELECT COUNT(*) FROM users;" -t 2>/dev/null)

if [ $? -eq 0 ] && [ -n "$DB_TEST" ]; then
    echo -e "${GREEN}✅ PASSED${NC}: Database Query"
    ((PASSED++))
    echo "Users count: $DB_TEST"
else
    echo -e "${RED}❌ FAILED${NC}: Database Query"
    ((FAILED++))
fi

echo -e "\n${BLUE}🔧 Step 7: Test Message Queue${NC}"
echo "============================"

# Test RabbitMQ queue status
RABBITMQ_TEST=$(docker exec uptimeping-rabbitmq rabbitmqctl list_queues 2>/dev/null)

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ PASSED${NC}: RabbitMQ Queue List"
    ((PASSED++))
    echo "Queues: $(echo "$RABBITMQ_TEST" | wc -l)"
else
    echo -e "${RED}❌ FAILED${NC}: RabbitMQ Queue List"
    ((FAILED++))
fi

echo -e "\n${BLUE}🔧 Step 8: Test Cache Operations${NC}"
echo "============================="

# Test Redis operations
REDIS_TEST=$(docker exec uptimeping-redis redis-cli set test_key "test_value" 2>/dev/null && docker exec uptimeping-redis redis-cli get test_key 2>/dev/null)

if [ $? -eq 0 ] && [ "$REDIS_TEST" = "test_value" ]; then
    echo -e "${GREEN}✅ PASSED${NC}: Redis Cache Operations"
    ((PASSED++))
else
    echo -e "${RED}❌ FAILED${NC}: Redis Cache Operations"
    ((FAILED++))
fi

echo -e "\n${BLUE}🔧 Step 9: Test Monitoring Stack${NC}"
echo "=============================="

# Test Prometheus targets
PROMETHEUS_TEST=$(wget --timeout=3 -qO- http://localhost:9090/api/v1/targets 2>/dev/null | grep -o '"health":"[^"]*"' | head -1)

if [ $? -eq 0 ] && [ -n "$PROMETHEUS_TEST" ]; then
    echo -e "${GREEN}✅ PASSED${NC}: Prometheus Targets"
    ((PASSED++))
    echo "Target status: $PROMETHEUS_TEST"
else
    echo -e "${RED}❌ FAILED${NC}: Prometheus Targets"
    ((FAILED++))
fi

# Test Grafana health
GRAFANA_TEST=$(wget --timeout=3 -qO- http://localhost:3000/api/health 2>/dev/null)

if [ $? -eq 0 ] && [ -n "$GRAFANA_TEST" ]; then
    echo -e "${GREEN}✅ PASSED${NC}: Grafana Health"
    ((PASSED++))
else
    echo -e "${RED}❌ FAILED${NC}: Grafana Health"
    ((FAILED++))
fi

echo -e "\n${BLUE}🔧 Step 10: Test Log Aggregation${NC}"
echo "=============================="

# Test Loki health
LOKI_TEST=$(wget --timeout=3 -qO- http://localhost:3100/ready 2>/dev/null)

if [ $? -eq 0 ] && [ -n "$LOKI_TEST" ]; then
    echo -e "${GREEN}✅ PASSED${NC}: Loki Health"
    ((PASSED++))
else
    echo -e "${RED}❌ FAILED${NC}: Loki Health"
    ((FAILED++))
fi

echo -e "\n${YELLOW}📋 End-to-End Test Results Summary${NC}"
echo "==================================="
echo -e "Total Tests: $((PASSED + FAILED))"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"

SUCCESS_RATE=$((PASSED * 100 / (PASSED + FAILED)))

if [ $SUCCESS_RATE -ge 80 ]; then
    echo -e "\n${GREEN}🎉 Excellent! System is highly functional (${SUCCESS_RATE}% success rate)${NC}"
    exit 0
elif [ $SUCCESS_RATE -ge 60 ]; then
    echo -e "\n${YELLOW}⚠️  Good! System is mostly functional (${SUCCESS_RATE}% success rate)${NC}"
    exit 0
else
    echo -e "\n${RED}❌ Poor! System has significant issues (${SUCCESS_RATE}% success rate)${NC}"
    exit 1
fi
