#!/bin/bash

# Скрипт для очистки портов UptimePingPlatform
# Использование: ./cleanup_ports.sh

echo "🧹 ОЧИСТКА ПОРТОВ UPTIMEPINGPLATFORM"

# Список портов приложений
APP_PORTS=(50051 50052 50053 50054 50055 50056 50057 8080 9090 3000)

# Функция для безопасного убийства процесса
kill_port() {
    local port=$1
    local pids=$(lsof -ti:$port 2>/dev/null)
    
    if [ ! -z "$pids" ]; then
        echo "🔥 Порт $port занят процессами: $pids"
        echo "$pids" | xargs kill -TERM 2>/dev/null || true
        sleep 2
        
        # Если процессы все еще живы, убиваем принудительно
        local remaining_pids=$(lsof -ti:$port 2>/dev/null)
        if [ ! -z "$remaining_pids" ]; then
            echo "⚡ Принудительное завершение процессов на порту $port: $remaining_pids"
            echo "$remaining_pids" | xargs kill -9 2>/dev/null || true
            sleep 1
        fi
        
        echo "✅ Порт $port освобожден"
    else
        echo "✅ Порт $port уже свободен"
    fi
}

# Очищаем порты приложений
echo ""
echo "📊 Очистка портов приложений:"
for port in "${APP_PORTS[@]}"; do
    kill_port $port
done

# Проверяем результат
echo ""
echo "🔍 ПРОВЕРКА РЕЗУЛЬТАТА:"
for port in "${APP_PORTS[@]}"; do
    if lsof -i:$port >/dev/null 2>&1; then
        echo "❌ Порт $port все еще занят"
        lsof -i:$port | grep LISTEN
    else
        echo "✅ Порт $port свободен"
    fi
done

echo ""
echo "🎉 ОЧИСТКА ЗАВЕРШЕНА!"
echo "💡 Теперь можно запускать сервисы UptimePingPlatform"
