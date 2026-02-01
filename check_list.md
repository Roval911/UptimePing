🔐 1. АУТЕНТИФИКАЦИЯ И АВТОРИЗАЦИЯ
POST /api/v1/auth/register
Описание: Регистрация нового пользователя
Принимает: {"email": "string", "password": "string", "tenant_name": "string"}
Ответ: {"access_token": "string", "refresh_token": "string", "tenant_id": "string"}
Статусы: 201 (создан), 400 (валидация), 500 (ошибка)


POST /api/v1/auth/login
Описание: Вход пользователя
Принимает: {"email": "string", "password": "string"}
Ответ: {"access_token": "string", "refresh_token": "string", "tenant_id": "string"}
Статусы: 200 (успех), 400 (валидация), 401 (неверные данные), 500 (ошибка)


POST /api/v1/auth/refresh
Описание: Обновление токена
Принимает: {"refresh_token": "string"}
Ответ: {"access_token": "string", "refresh_token": "string"}
Статусы: 200 (успех), 400 (валидация), 401 (недействительный), 500 (ошибка)


POST /api/v1/auth/logout
Описание: Выход из системы
Принимает: {"access_token": "string"}
Ответ: {"message": "Logged out successfully"}
Статусы: 200 (успех), 400 (валидация), 500 (ошибка)


POST /api/v1/auth/validate
Описание: Валидация токена
Принимает: {"access_token": "string"}
Ответ: {"user_id": "string", "tenant_id": "string", "email": "string", "is_admin": "bool", "expires_at": "number"}
Статусы: 200 (валиден), 400 (невалидный), 401 (просрочен), 500 (ошибка)


GET /api/v1/auth/api-keys
Описание: Управление API ключами
Принимает: Нет данных (GET)
Ответ: {"id": "string", "key": "string", "secret": "string", "name": "string"}
Статусы: 200 (успех), 405 (метод не разрешен)


🔍 2. ПРОВЕРКИ (CHECKS)

GET /api/v1/checks
Описание: Получение списка проверок
Принимает: Query параметры (page, page_size)
Ответ: {"checks": [], "total": "number"}
Статусы: 200 (успех), 401 (неавторизован), 500 (ошибка)


POST /api/v1/checks
Описание: Создание новой проверки
Принимает: {"name": "string", "type": "string", "target": "string", "interval": "number", "timeout": "number", "config": {}}
Ответ: {"success": true, "message": "Check created", "check": {}}
Статусы: 201 (создан), 400 (валидация), 401 (неавторизован), 500 (ошибка)


GET /api/v1/checks/{id}
Описание: Получение конкретной проверки
Принимает: ID проверки в URL
Ответ: {"success": true, "check": {}}
Статусы: 200 (успех), 400 (невалидный ID), 401 (неавторизован), 403 (доступ запрещен), 404 (не найден), 500 (ошибка)


PUT /api/v1/checks/{id}
Описание: Обновление проверки
Принимает: ID проверки в URL, тело с полями для обновления
Ответ: {"success": true, "message": "Check updated", "check": {}}
Статусы: 200 (успех), 400 (валидация), 401 (неавторизован), 403 (доступ запрещен), 404 (не найден), 500 (ошибка)


DELETE /api/v1/checks/{id}
Описание: Удаление проверки
Принимает: ID проверки в URL
Ответ: {"success": true, "message": "Check deleted"}
Статусы: 200 (успех), 400 (невалидный ID), 401 (неавторизован), 403 (доступ запрещен), 404 (не найден), 500 (ошибка)


📅 3. РАСПИСАНИЯ (SCHEDULES)

GET /api/v1/schedules
Описание: Получение списка расписаний
Принимает: Нет данных
Ответ: {"success": true, "schedules": [], "total": "number"}
Статусы: 200 (успех), 401 (неавторизован), 500 (ошибка)

POST /api/v1/schedules
Описание: Создание расписания
Принимает: {"cron_expression": "string"}
Ответ: {"success": true, "message": "Check scheduled", "schedule": {}}
Статусы: 201 (создано), 400 (валидация), 401 (неавторизован), 500 (ошибка)

GET /api/v1/schedules/{id}
Описание: Получение расписания проверки
Принимает: ID проверки в URL
Ответ: {"success": true, "schedule": {}}
Статусы: 200 (успех), 400 (невалидный ID), 401 (неавторизован), 500 (ошибка)

DELETE /api/v1/schedules/{id}
Описание: Удаление расписания
Принимает: ID проверки в URL
Ответ: {"success": true, "message": "Check unscheduled"}
Статусы: 200 (успех), 400 (невалидный ID), 401 (неавторизован), 500 (ошибка)

⚡ 4. CORE SERVICE ОПЕРАЦИИ

POST /api/v1/core
Описание: Выполнение проверки
Принимает: ID проверки в URL
Ответ: {"success": boolean, "execution_id": "string", "duration_ms": "number", "status_code": "number", "error": "string", "checked_at": "string"}
Статусы: 200 (успех), 400 (невалидный ID), 401 (неавторизован), 500 (ошибка)

GET /api/v1/core/{id}/status
Описание: Получение статуса проверки
Принимает: ID проверки в URL
Ответ: {"check_id": "string", "is_healthy": boolean, "response_time_ms": "number", "last_checked_at": "string"}
Статусы: 200 (успех), 400 (невалидный ID), 401 (неавторизован), 500 (ошибка)

GET /api/v1/core/{id}/history
Описание: Получение истории выполнения проверки
Принимает: ID проверки в URL, query параметры (page, page_size)
Ответ: {"executions": [], "page": "number", "page_size": "number", "total": "number"}
Статусы: 200 (успех), 400 (невалидный ID), 401 (неавторизован), 500 (ошибка)

🔧 5. FORGE SERVICE

POST /api/v1/forge/generate
Описание: Генерация конфигурации
Принимает: {"proto_content": "string", "action": "generate_config", "options": {}}
Ответ: {"success": true, "message": "Configuration generated successfully", "config_yaml": "string", "check_config": {}}
Статусы: 200 (успех), 400 (валидация), 401 (неавторизован), 500 (ошибка)

POST /api/v1/forge/parse
Описание: Парсинг proto файла
Принимает: {"proto_content": "string", "action": "parse_proto", "file_name": "string"}
Ответ: {"success": true, "message": "Proto parsed successfully", "service_info": {}, "is_valid": boolean, "warnings": []}
Статусы: 200 (успех), 400 (валидация), 401 (неавторизован), 500 (ошибка)

POST /api/v1/forge/code
Описание: Генерация кода
Принимает: {"proto_content": "string", "action": "generate_code", "options": {"language": "string", "framework": "string", "template": "string"}}
Ответ: {"success": true, "message": "Code generated successfully", "code": "string", "filename": "string", "language": "string"}
Статусы: 200 (успех), 400 (валидация), 401 (неавторизован), 500 (ошибка)

POST /api/v1/forge/validate
Описание: Валидация proto файла
Принимает: {"proto_content": "string", "action": "validate_proto"}
Ответ: {"success": true, "message": "Proto validated successfully", "is_valid": boolean, "errors": [], "warnings": []}
Статусы: 200 (успех), 400 (валидация), 401 (неавторизован), 500 (ошибка)

📊 6. METRICS SERVICE

GET /api/v1/metrics
Описание: Получение метрик
Принимает: Query параметр service_name
Ответ: {"success": true, "metrics": [], "total": "number"}
Статусы: 200 (успех), 401 (неавторизован), 500 (ошибка)

POST /api/v1/metrics/collect
Описание: Сбор метрик
Принимает: {"metrics": []}
Ответ: {"success": boolean, "metrics_count": "number", "collected_at": "string"}
Статусы: 200 (успех), 400 (валидация), 401 (неавторизован), 500 (ошибка)

🚨 7. INCIDENT SERVICE

GET /api/v1/incidents
Описание: Получение списка инцидентов
Принимает: Нет данных
Ответ: {"incidents": [], "total": "number"}
Статусы: 200 (успех), 401 (требуются права), 500 (ошибка)

POST /api/v1/incidents
Описание: Создание инцидента
Принимает: {"title": "string", "description": "string", "severity": "string"}
Ответ: {"success": true, "message": "Incident created", "incident": {}}
Статусы: 201 (создан), 400 (валидация), 401 (неавторизован), 500 (ошибка)

GET /api/v1/incidents/{id}
Описание: Получение инцидента
Принимает: ID инцидента в URL
Ответ: {"success": true, "incident": {}}
Статусы: 200 (успех), 400 (невалидный ID), 401 (неавторизован), 404 (не найден), 500 (ошибка)

PUT /api/v1/incidents/{id}
Описание: Разрешение инцидента
Принимает: ID инцидента в URL, {"resolution": "string", "resolved_by": "string"}
Ответ: {"success": boolean, "message": "Incident resolved"}
Статусы: 200 (успех), 400 (валидация), 401 (неавторизован), 404 (не найден), 500 (ошибка)

📢 8. NOTIFICATION SERVICE

GET /api/v1/notifications
Описание: Отправка уведомления
Принимает: {"message": "string", "channel": "string", "priority": "string"}
Ответ: {"success": boolean, "results": []}
Статусы: 200 (успех), 400 (валидация), 401 (неавторизован), 500 (ошибка)

GET /api/v1/notifications/channels
Описание: Получение каналов уведомлений
Принимает: Нет данных
Ответ: {"channels": [], "total": "number"}
Статусы: 200 (успех), 401 (неавторизован), 500 (ошибка)

POST /api/v1/notifications/channels
Описание: Создание канала уведомлений
Принимает: {"name": "string", "type": "string", "config": {}}
Ответ: {"success": true, "message": "Notification channel created", "channel": {}}
Статусы: 201 (создан), 400 (валидация), 401 (неавторизован), 500 (ошибка)

⚙️ 9. CONFIG SERVICE
GET /api/v1/config
Описание: Получение конфигурации
Принимает: Нет данных
Ответ: {"config": {"version": "string", "environment": "string"}}
Статусы: 200 (успех), 401 (требуются права), 500 (ошибка)

🏥 10. HEALTH CHECKS
GET /health
Описание: Базовый health check
Принимает: Нет данных
Ответ: {"status": "healthy", "timestamp": "string", "version": "string"}
Статусы: 200 (всегда здоров)

GET /ready
Описание: Проверка готовности сервиса
Принимает: Нет данных
Ответ: {"status": "ready", "timestamp": "string", "version": "string"}
Статусы: 200 (готов), 503 (не готов)

GET /live
Описание: Проверка жизнеспособности
Принимает: Нет данных
Ответ: {"status": "alive", "timestamp": "string", "version": "string"}
Статусы: 200 (жив), 503 (не жив)

GET /api/v1/auth/health
Описание: Health check Auth Service
Принимает: Нет данных
Ответ: Health ответ от Auth Service
Статусы: 200 (здоров), 503 (недоступен)

GET /api/v1/scheduler/health
Описание: Health check Scheduler Service
Принимает: Нет данных
Ответ: Health ответ от Scheduler Service
Статусы: 200 (здоров), 503 (недоступен)

GET /api/v1/core/health
Описание: Health check Core Service
Принимает: Нет данных
Ответ: Health ответ от Core Service
Статусы: 200 (здоров), 503 (недоступен)

📋 ИТОГО: 32 ЭНДПОИНТА
