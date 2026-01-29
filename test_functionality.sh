#!/bin/bash

echo "🚀 Starting Functional Tests for UptimePingPlatform"
echo "=================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results
PASSED=0
FAILED=0

# Function to run test
run_test() {
    local test_name="$1"
    local command="$2"
    
    echo -e "\n📋 Testing: $test_name"
    echo "Command: $command"
    
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
    echo "Command: $command"
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

echo -e "\n${YELLOW}🐳 Docker Container Tests${NC}"
echo "=============================="

run_test "API Gateway Container Running" "docker ps | grep uptimeping-api-gateway | grep -q Up"
run_test "Auth Service Container Running" "docker ps | grep uptimeping-auth-service | grep -q Up"
run_test "Core Service Container Running" "docker ps | grep uptimeping-core-service | grep -q Up"
run_test "Scheduler Service Container Running" "docker ps | grep uptimeping-scheduler-service | grep -q Up"
run_test "PostgreSQL Container Running" "docker ps | grep uptimeping-postgres | grep -q Up"
run_test "Redis Container Running" "docker ps | grep uptimeping-redis | grep -q Up"
run_test "RabbitMQ Container Running" "docker ps | grep uptimeping-rabbitmq | grep -q Up"
run_test "Prometheus Container Running" "docker ps | grep uptimeping-prometheus | grep -q Up"
run_test "Grafana Container Running" "docker ps | grep uptimeping-grafana | grep -q Up"

echo -e "\n${YELLOW}🌐 API Gateway Tests (Port 8080)${NC}"
echo "====================================="

test_with_output "API Gateway Health Check" "wget -qO- http://localhost:8080/health"
test_with_output "API Gateway Ready Check" "wget -qO- http://localhost:8080/ready"
test_with_output "API Gateway Live Check" "wget -qO- http://localhost:8080/live"
run_test "API Gateway Metrics Endpoint" "wget --timeout=1 -qO- http://localhost:8080/metrics > /dev/null"

echo -e "\n${YELLOW}📅 Scheduler Service Tests (Port 50052)${NC}"
echo "========================================"

test_with_output "Scheduler Health Check" "docker exec uptimeping-scheduler-service wget -qO- http://localhost:50052/health"
run_test "Scheduler Ready Check" "docker exec uptimeping-scheduler-service wget --timeout=1 -qO- http://localhost:50052/ready > /dev/null"
run_test "Scheduler Metrics Endpoint" "docker exec uptimeping-scheduler-service wget --timeout=1 -qO- http://localhost:50052/metrics > /dev/null"

echo -e "\n${YELLOW}🔐 Auth Service Tests (Port 50051)${NC}"
echo "======================================"

run_test "Auth Service Container Healthy" "docker ps | grep uptimeping-auth-service | grep -q healthy"

echo -e "\n${YELLOW}⚡ Core Service Tests (Port 50054)${NC}"
echo "====================================="

run_test "Core Service Container Healthy" "docker ps | grep uptimeping-core-service | grep -q healthy"

echo -e "\n${YELLOW}🗄️ Infrastructure Tests${NC}"
echo "========================"

run_test "PostgreSQL Connection" "docker exec uptimeping-postgres pg_isready -h localhost -p 5432"
run_test "Redis Connection" "docker exec uptimeping-redis redis-cli ping"
run_test "RabbitMQ Connection" "docker exec uptimeping-rabbitmq rabbitmq-diagnostics -q check_running"

echo -e "\n${YELLOW}📊 Monitoring Tests${NC}"
echo "==================="

run_test "Prometheus Target Endpoint" "wget --timeout=1 -qO- http://localhost:9090/api/v1/targets > /dev/null"
run_test "Grafana Health Endpoint" "wget --timeout=1 -qO- http://localhost:3000/api/health > /dev/null"

echo -e "\n${YELLOW}🔗 Integration Tests${NC}"
echo "===================="

# Test API Gateway can reach other services
run_test "API Gateway to Auth Service" "wget --timeout=2 -qO- http://localhost:8080/api/v1/auth/health > /dev/null"
run_test "API Gateway to Scheduler Service" "wget --timeout=2 -qO- http://localhost:8080/api/v1/scheduler/health > /dev/null"

echo -e "\n${YELLOW}📋 Test Results Summary${NC}"
echo "========================="
echo -e "Total Tests: $((PASSED + FAILED))"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo -e "\n${GREEN}🎉 All tests passed! System is fully functional.${NC}"
    exit 0
else
    echo -e "\n${RED}⚠️  Some tests failed. Please check the system.${NC}"
    exit 1
fi
