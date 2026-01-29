# Makefile для UptimePing Platform

# Переменные
PROJECT_NAME = UptimePing Platform
VERSION = 0.1.0
BUILD_TIME = $(shell date +%Y-%m-%d-%H:%M:%S)
GIT_COMMIT = $(shell git rev-parse --short HEAD)
GO_VERSION = 1.24

# Пути
SCRIPTS_DIR = scripts
MIGRATIONS_DIR = migrations
SERVICES_DIR = services
DEPLOYMENTS_DIR = deployments

# Docker образы
REGISTRY = uptimeping
DOCKER_TAG = $(VERSION)-$(GIT_COMMIT)

# Цели по умолчанию
.PHONY: help build test start stop clean migrate init-db proto
.PHONY: build-all build-linux build-macos build-windows
.PHONY: package-deb package-rpm package-homebrew package-chocolatey
.PHONY: docker-build docker-push docker-tag

proto:
	@echo "Генерация кода из proto файлов..."
	buf generate

setup:
	@echo "Настройка окружения..."
	${SCRIPTS_DIR}/setup-env.sh

init:
	@echo "Инициализация базы данных..."
	${SCRIPTS_DIR}/init-db.sh

migrate:
	@echo "Применение миграций..."
	${SCRIPTS_DIR}/migrate.sh

start:
	@echo "Запуск платформы..."
	docker-compose up -d

stop:
	@echo "Остановка платформы..."
	docker-compose down

restart:
	make stop
	make start

logs:
	@echo "Логи платформы (нажмите Ctrl+C для выхода)..."
	docker-compose logs -f

logs-service:
	@echo "Логи сервиса $(service) (нажмите Ctrl+C для выхода)..."
	docker-compose logs -f $(service)

ps:
	@echo "Состояние сервисов:"
	docker-compose ps

config:
	@echo "Конфигурация docker-compose:"
	docker-compose config

pull:
	@echo "Обновление образов..."
	docker-compose pull

# Сборка для разных платформ
build-all: build-linux build-macos build-windows

build-linux:
	@echo "Сборка для Linux..."
	@for service in api-gateway auth-service core-service scheduler-service metrics-service incident-manager notification-service forge-service cli-service; do \
		echo "Сборка $$service для Linux..."; \
		cd $(SERVICES_DIR)/$$service && \
		GOOS=linux GOARCH=amd64 go build -ldflags="-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)" -o dist/linux/$$service cmd/main.go; \
	done

build-macos:
	@echo "Сборка для macOS..."
	@for service in api-gateway auth-service core-service scheduler-service metrics-service incident-manager notification-service forge-service cli-service; do \
		echo "Сборка $$service для macOS..."; \
		cd $(SERVICES_DIR)/$$service && \
		GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)" -o dist/macos/$$service cmd/main.go; \
	done

build-windows:
	@echo "Сборка для Windows..."
	@for service in api-gateway auth-service core-service scheduler-service metrics-service incident-manager notification-service forge-service cli-service; do \
		echo "Сборка $$service для Windows..."; \
		cd $(SERVICES_DIR)/$$service && \
		GOOS=windows GOARCH=amd64 go build -ldflags="-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)" -o dist/windows/$$service.exe cmd/main.go; \
	done

# Docker сборка
docker-build:
	@echo "Сборка Docker образов..."
	@for service in api-gateway auth-service core-service scheduler-service metrics-service incident-manager notification-service forge-service cli-service; do \
		echo "Сборка Docker образа для $$service..."; \
		docker build -t $(REGISTRY)/$$service:$(DOCKER_TAG) -f $(SERVICES_DIR)/$$service/Dockerfile .; \
	done

docker-push:
	@echo "Отправка Docker образов..."
	@for service in api-gateway auth-service core-service scheduler-service metrics-service incident-manager notification-service forge-service cli-service; do \
		echo "Отправка Docker образа для $$service..."; \
		docker push $(REGISTRY)/$$service:$(DOCKER_TAG); \
	done

docker-tag:
	@echo "Тегирование Docker образов..."
	@for service in api-gateway auth-service core-service scheduler-service metrics-service incident-manager notification-service forge-service cli-service; do \
		docker tag $(REGISTRY)/$$service:$(DOCKER_TAG) $(REGISTRY)/$$service:latest; \
	done

# Упаковка для Linux
package-deb:
	@echo "Создание DEB пакетов..."
	@mkdir -p dist/deb
	@for service in api-gateway auth-service core-service scheduler-service metrics-service incident-manager notification-service forge-service cli-service; do \
		echo "Создание DEB пакета для $$service..."; \
		cd $(SERVICES_DIR)/$$service && \
		mkdir -p ../../dist/deb/$$service/DEBIAN && \
		cp ../../scripts/deb/control ../../dist/deb/$$service/DEBIAN/ && \
		cp ../../scripts/deb/postinst ../../dist/deb/$$service/DEBIAN/ && \
		cp ../../scripts/deb/prerm ../../dist/deb/$$service/DEBIAN/ && \
		mkdir -p ../../dist/deb/$$service/usr/bin && \
		cp ../../dist/linux/$$service ../../dist/deb/$$service/usr/bin/ && \
		dpkg-deb --build ../../dist/deb/$$service; \
	done

package-rpm:
	@echo "Создание RPM пакетов..."
	@mkdir -p dist/rpm
	@for service in api-gateway auth-service core-service scheduler-service metrics-service incident-manager notification-service forge-service cli-service; do \
		echo "Создание RPM пакета для $$service..."; \
		cd $(SERVICES_DIR)/$$service && \
		mkdir -p ../../dist/rpm/$$service && \
		cp ../../scripts/rpm/$$service.spec ../../dist/rpm/$$service/ && \
		cp ../../dist/linux/$$service ../../dist/rpm/$$service/ && \
		rpmbuild --define "_topdir $$(pwd)/dist/rpm" -bb ../../dist/rpm/$$service/$$service.spec; \
	done

# Упаковка для macOS
package-homebrew:
	@echo "Создание Homebrew formula..."
	@mkdir -p dist/homebrew
	@cat > dist/homebrew/uptimeping.rb << 'EOF'
class Uptimeping < Formula
  desc "UptimePing Platform - Микросервисная платформа для мониторинга доступности сервисов"
  homepage "https://github.com/uptimeping/uptimeping"
  url "https://github.com/uptimeping/uptimeping/archive/v$(VERSION).tar.gz"
  sha256 "$(shell sha256sum dist/uptimeping-$(VERSION).tar.gz | cut -d' ' -f1)"
  license "MIT"
  head "https://github.com/uptimeping/uptimeping.git", branch: "main"

  depends_on "go@$(GO_VERSION)"

  def install
    ENV["GOPATH"] = "$(HOMEBREW_PREFIX)/opt/go"
    ENV["GOOS"] = "darwin"
    ENV["GOARCH"] = "amd64"
    
    system "go", "build", "-ldflags", "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)", "-o", "uptimeping"
    bin.install "uptimeping"
  end

  test do
    system "#{bin}/uptimeping", "--version"
  end
end
EOF

# Упаковка для Windows
package-chocolatey:
	@echo "Создание Chocolatey пакета..."
	@mkdir -p dist/chocolatey
	@cat > dist/chocolatey/uptimeping.nuspec << 'EOF'
<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd">
  <metadata>
    <id>uptimeping</id>
    <version>$(VERSION)</version>
    <packageSourceUrl>https://github.com/uptimeping/uptimeping</packageSourceUrl>
    <owners>UptimePing</owners>
    <title>UptimePing Platform</title>
    <authors>UptimePing Team</authors>
    <projectUrl>https://github.com/uptimeping/uptimeping</projectUrl>
    <iconUrl>https://raw.githubusercontent.com/uptimeping/uptimeping/main/logo.png</iconUrl>
    <copyright>Copyright 2024 UptimePing</copyright>
    <licenseUrl>https://github.com/uptimeping/uptimeping/blob/main/LICENSE</licenseUrl>
    <requireLicenseAcceptance>false</requireLicenseAcceptance>
    <projectSourceUrl>https://github.com/uptimeping/uptimeping</projectSourceUrl>
    <docsUrl>https://docs.uptimeping.local</docsUrl>
    <bugTrackerUrl>https://github.com/uptimeping/uptimeping/issues</bugTrackerUrl>
    <tags>uptimeping monitoring microservices</tags>
    <summary>UptimePing Platform - Микросервисная платформа для мониторинга доступности сервисов</summary>
    <description>UptimePing Platform - это микросервисная платформа для мониторинга доступности сервисов с поддержкой различных типов проверок, уведомлений и метрик.</description>
  </metadata>
  <files>
    <file src="dist/windows\*.exe" target="tools" />
  </files>
</package>
EOF

# Тестирование
test:
	@echo "Запуск тестов..."
	@for service in api-gateway auth-service core-service scheduler-service metrics-service incident-manager notification-service forge-service cli-service; do \
		echo "Тестирование $$service..."; \
		cd $(SERVICES_DIR)/$$service && go test -v ./...; \
	done

test-coverage:
	@echo "Запуск тестов с покрытием..."
	@for service in api-gateway auth-service core-service scheduler-service metrics-service incident-manager notification-service forge-service cli-service; do \
		echo "Тестирование $$service с покрытием..."; \
		cd $(SERVICES_DIR)/$$service && go test -v -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html; \
	done

# Очистка
clean:
	@echo "Очистка..."
	docker-compose down -v --remove-orphans
	rm -rf .env dist/*
	rm -f coverage.out coverage.html

# Алиасы
up: start
down: stop
status: ps

# Разработка
dev:
	@echo "Запуск в режиме разработки..."
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d

dev-logs:
	@echo "Логи в режиме разработки..."
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml logs -f

# Производство
prod:
	@echo "Запуск в режиме производства..."
	docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Мониторинг
monitoring:
	@echo "Запуск только мониторинга..."
	docker-compose up -d prometheus grafana loki promtail

monitoring-logs:
	@echo "Логи мониторинга..."
	docker-compose logs -f prometheus grafana loki promtail

# База данных
db:
	@echo "Запуск только базы данных..."
	docker-compose up -d postgres redis rabbitmq

db-reset:
	@echo "Сброс базы данных..."
	docker-compose down -v
	docker-compose up -d postgres redis rabbitmq
	make migrate

# Резервное копирование
backup:
	@echo "Создание резервной копии..."
	mkdir -p backups
	docker exec uptimeping-postgres pg_dump -U uptimeping uptimeping > backups/backup-$(shell date +%Y%m%d-%H%M%S).sql

restore:
	@echo "Восстановление из резервной копии..."
	@read -p "Введите имя файла резервной копии: " backup_file; \
	if [ -f "backups/$$backup_file" ]; then \
		docker exec -i uptimeping-postgres psql -U uptimeping uptimeping < backups/$$backup_file; \
	else \
		echo "Файл не найден: backups/$$backup_file"; \
	fi

# Вспомогательные цели
help:
	@echo "Доступные цели:"
	@echo "  build-all     - Сборка всех сервисов для всех платформ"
	@echo "  build-linux   - Сборка для Linux"
	@echo "  build-macos    - Сборка для macOS"
	@echo "  build-windows  - Сборка для Windows"
	@echo "  docker-build  - Сборка Docker образов"
	@echo "  docker-push   - Отправка Docker образов"
	@echo "  package-deb   - Создание DEB пакетов"
	@echo "  package-rpm   - Создание RPM пакетов"
	@echo "  package-homebrew - Создание Homebrew formula"
	@echo "  package-chocolatey - Создание Chocolatey пакета"
	@echo "  test          - Запуск тестов"
	@echo "  test-coverage - Тесты с покрытием"
	@echo "  start         - Запуск всех сервисов"
	@echo "  stop          - Остановка всех сервисов"
	@echo "  restart       - Перезапуск всех сервисов"
	@echo "  logs          - Просмотр логов"
	@echo "  dev           - Запуск в режиме разработки"
	@echo "  prod          - Запуск в режиме производства"
	@echo "  monitoring    - Запуск только мониторинга"
	@echo "  db            - Запуск только базы данных"
	@echo "  db-reset      - Сброс базы данных"
	@echo "  backup        - Создание резервной копии"
	@echo "  restore       - Восстановление из резервной копии"
	@echo "  clean         - Очистка"
	@echo "  help          - Показать эту справку"
