#!/bin/bash

# Скрипт для корректной остановки всех сервисов UptimePingPlatform
# Использование: ./graceful_stop.sh

echo "🛑 КОРРЕКТНАЯ ОСТАНОВКА СЕРВИСОВ UPTIMEPINGPLATFORM"

# Функция для отправки SIGTERM
graceful_stop() {
    local service_name=$1
    local pids=$(pgrep -f "$service_name" 2>/dev/null)
    
    if [ ! -z "$pids" ]; then
        echo "🔄 Остановка $service_name (PID: $pids)..."
        echo "$pids" | xargs kill -TERM 2>/dev/null || true
        
        # Даем время на корректную остановку
        sleep 5
        
        # Проверяем, завершились ли процессы
        local remaining_pids=$(pgrep -f "$service_name" 2>/dev/null)
        if [ ! -z "$remaining_pids" ]; then
            echo "⚡ Принудительная остановка $service_name (PID: $remaining_pids)..."
            echo "$remaining_pids" | xargs kill -9 2>/dev/null || true
        else
            echo "✅ $service_name корректно остановлен"
        fi
    else
        echo "✅ $service_name не запущен"
    fi
}

# Останавливаем сервисы в правильном порядке
echo ""
echo "📋 Остановка сервисов:"

# 1. API Gateway (внешний сервис)
graceful_stop "api-gateway"

# 2. Auth Service
graceful_stop "auth-service"

# 3. Scheduler Service
graceful_stop "scheduler-service"

# 4. Core Service (если запущен)
graceful_stop "core-service"

# 5. Metrics Service (если запущен)
graceful_stop "metrics-service"

# 6. Остальные сервисы
for service in "notification-service" "incident-manager" "forge-service"; do
    graceful_stop "$service"
done

echo ""
echo "🔍 ПРОВЕРКА ПОРТОВ:"
APP_PORTS=(50051 50052 50053 50054 50055 50056 50057 8080 9090 3000)

for port in "${APP_PORTS[@]}"; do
    if lsof -i:$port >/dev/null 2>&1; then
        echo "❌ Порт $port все еще занят"
    else
        echo "✅ Порт $port свободен"
    fi
done

echo ""
echo "🎉 ВСЕ СЕРВИСЫ ОСТАНОВЛЕНЫ!"
