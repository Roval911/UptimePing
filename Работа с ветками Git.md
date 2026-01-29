
## Модель ветвления

Используем упрощенную модель Git Flow:

```
main (production)
  │
  ├── develop (development) [опционально]
  │     │
  │     ├── feature/*
  │     ├── bugfix/*
  │     └── refactor/*
  │
  └── hotfix/*
```

## Основные ветки

### main
- **Назначение**: Продакшен-готовая версия
- **Защита**: Только через Merge Request
- **Правила**:
  - Всегда в рабочем состоянии
  - Каждый коммит должен быть протестирован
  - Только мерджи из `develop` или `hotfix/*`
  - Теги версий создаются здесь

### develop (опционально)
- **Назначение**: Ветка разработки, интеграция фич
- **Защита**: Только через Merge Request
- **Правила**:
  - Всегда в рабочем состоянии
  - Интеграция фич перед релизом
  - Если не используется, фичи мерджатся напрямую в `main`

## Ветки разработки

### feature/\*
- **Назначение**: Разработка новой функциональности
- **Создание**: От `develop` или `main`
- **Название**: `feature/<service-name>-<description>`
- **Примеры**:
  - `feature/auth-service-init`
  - `feature/core-http-checker`
  - `feature/gateway-middleware`
- **Правила**:
  - Одна ветка = одна фича
  - Частые коммиты
  - Мердж обратно в `develop`/`main` через MR
  - Удаление после мерджа

### bugfix/\*
- **Назначение**: Исправление багов
- **Создание**: От `develop` или `main`
- **Название**: `bugfix/<service-name>-<description>`
- **Примеры**:
  - `bugfix/auth-jwt-validation`
  - `bugfix/core-timeout-handling`
- **Правила**:
  - Одна ветка = один баг
  - Включать номер issue (если есть)
  - Мердж обратно в `develop`/`main` через MR
  - Удаление после мерджа

### hotfix/\*
- **Назначение**: Критические исправления для продакшена
- **Создание**: От `main`
- **Название**: `hotfix/<description>`
- **Примеры**:
  - `hotfix/critical-security-patch`
  - `hotfix/db-connection-fix`
- **Правила**:
  - Только для критических багов в продакшене
  - Мердж в `main` и `develop` (если есть)
  - Создание тега версии
  - Удаление после мерджа

### refactor/\*
- **Назначение**: Рефакторинг кода
- **Создание**: От `develop` или `main`
- **Название**: `refactor/<service-name>-<description>`
- **Примеры**:
  - `refactor/config-validation-logic`
  - `refactor/core-checker-interface`
- **Правила**: Аналогично `feature/*`

### docs/\*
- **Назначение**: Изменения в документации
- **Создание**: От `develop` или `main`
- **Название**: `docs/<description>`
- **Примеры**:
  - `docs/api-documentation`
  - `docs/architecture-update`
- **Правила**: Может мерджиться быстрее, без code review (по усмотрению)

## Workflow

### Создание feature ветки

```bash
# Обновить основную ветку
git checkout main
git pull origin main

# Создать feature ветку
git checkout -b feature/auth-service-init

# Работа над фичей
git add .
git commit -m "feat(auth): initialize auth service structure"

# Push ветки
git push origin feature/auth-service-init
```

### Работа над фичей

```bash
# Регулярно синхронизировать с основной веткой
git checkout main
git pull origin main
git checkout feature/auth-service-init
git rebase main  # или git merge main

# Решить конфликты если есть
# Продолжить работу
```

### Завершение фичи

```bash
# Убедиться что все коммиты сделаны
git status

# Push финальных изменений
git push origin feature/auth-service-init

# Создать Merge Request в GitLab
# После мерджа удалить ветку
git checkout main
git pull origin main
git branch -d feature/auth-service-init
git push origin --delete feature/auth-service-init
```

### Hotfix workflow

```bash
# Создать hotfix от main
git checkout main
git pull origin main
git checkout -b hotfix/critical-security-patch

# Исправить баг
git add .
git commit -m "fix(auth): fix critical security vulnerability"

# Мердж в main
git checkout main
git merge hotfix/critical-security-patch
git tag -a v1.0.1 -m "Hotfix: critical security patch"
git push origin main --tags

# Мердж в develop (если есть)
git checkout develop
git merge hotfix/critical-security-patch
git push origin develop

# Удалить hotfix ветку
git branch -d hotfix/critical-security-patch
```

## Правила именования

### Формат
```
<type>/<scope>-<description>
```

### Типы
- `feature` - новая функциональность
- `bugfix` - исправление бага
- `hotfix` - критическое исправление
- `refactor` - рефакторинг
- `docs` - документация
- `test` - тесты
- `chore` - вспомогательные задачи

### Scope
- Имя сервиса: `auth`, `config`, `core`, `scheduler`, `notification`, `forge`, `gateway`, `metrics`
- Или общий: `infra`, `ci`, `docs`

### Description
- В нижнем регистре
- Разделение через дефис
- Краткое описание (2-4 слова)

### Примеры хороших названий
```
feature/auth-service-init
feature/core-http-checker
bugfix/jwt-validation-error
hotfix/critical-db-connection
refactor/config-validation-logic
docs/api-documentation
test/auth-unit-tests
chore/update-dependencies
```

### Примеры плохих названий
```
feature/new-stuff
fix/bug
my-feature
auth-service
feature/auth_service_init  # используйте дефис, не подчеркивание
```

## Merge Request процесс

### Создание MR

1. **Заголовок MR**:
   2. Используйте тот же формат что и коммиты
   3. Пример: `feat(auth): implement user registration`

2. **Описание MR**:
```markdown
## Описание
Реализована регистрация пользователей в Auth Service

## Изменения
- Добавлен endpoint POST /api/v1/auth/register
- Реализована валидация входных данных
- Добавлено хеширование паролей
- Написаны unit тесты

## Связанные задачи
Closes #123

## Чеклист
- [x] Код компилируется
- [x] Тесты проходят
- [x] Документация обновлена
- [x] Нет линтер ошибок
```

3. **Labels**:
   2. `feature`, `bugfix`, `hotfix`, etc.
   3. `auth-service`, `core-service`, etc.
   4. `ready-for-review`

### Code Review

1. **Минимум 1 одобрение** для мерджа
2. **Проверки**:
   3. Соответствие архитектуре
   4. Наличие тестов
   5. Обработка ошибок
   6. Логирование
   7. Документация
   8. Производительность

3. **После одобрения**:
   2. Автор мерджит MR
   3. Удаляет ветку

### Squash commits (опционально)

Если в ветке много мелких коммитов, можно использовать squash при мердже:
- В GitLab: включить опцию "Squash commits"
- Или вручную: `git rebase -i main` перед созданием MR

## Защита веток

### main
- **Защита**: Включена
- **Разрешить мердж**: Только через MR
- **Требовать одобрение**: Да (минимум 1)
- **Разрешить force push**: Нет
- **Разрешить удаление**: Нет

### develop (если используется)
- **Защита**: Включена
- **Разрешить мердж**: Только через MR
- **Требовать одобрение**: Да (минимум 1)
- **Разрешить force push**: Нет

## Теги версий

### Формат
```
v<major>.<minor>.<patch>
```

### Примеры
```
v1.0.0  # Первый релиз
v1.0.1  # Hotfix
v1.1.0  # Новая фича
v2.0.0  # Breaking changes
```

### Создание тега
```bash
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0
```

## Best Practices

1. **Частые коммиты**: Коммитьте после каждого логического этапа
2. **Синхронизация**: Регулярно синхронизируйте ветку с основной
3. **Одна задача**: Одна ветка = одна задача
4. **Чистая история**: Используйте rebase для чистой истории (если команда согласна)
5. **Удаление веток**: Удаляйте ветки после мерджа
6. **Описательные названия**: Названия веток должны быть понятными
7. **MR рано**: Создавайте MR как можно раньше (draft), чтобы получить feedback
