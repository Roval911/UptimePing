# Scheduler Service

Сервис для планирования и выполнения проверок доступности систем.

## Архитектура

### Domain Layer
- `models.go` - доменные модели (Check, Schedule, Task)
- `models_test.go` - тесты моделей

### Repository Layer  
- `check_repository.go` - интерфейс для работы с проверками в БД
- `scheduler_repository.go` - интерфейс для работы с планировщиком

### Use Case Layer
- `check_usecase.go` - бизнес-логика управления проверками
- `check_usecase_test.go` - тесты use cases

## Функциональность

### Управление проверками
- **CreateCheck** - создание новой проверки с валидацией
- **UpdateCheck** - обновление существующей проверки  
- **DeleteCheck** - удаление проверки

### Поддерживаемые типы проверок
- HTTP/HTTPS
- GRPC
- GraphQL
- TCP

### Валидация
- Базовая валидация параметров
- Специфичная валидация для каждого типа проверки
- Проверка конфигурации (методы, порты, статусы и т.д.)

## Запуск тестов

```bash
go test ./internal/usecase -v
go test ./internal/domain -v
```

## Зависимости

- github.com/google/uuid v1.6.0
- github.com/stretchr/testify v1.11.1
