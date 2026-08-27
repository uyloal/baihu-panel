# Variables
BINARY=bin/baihu
GOBUILD=go build
GOCLEAN=go clean
GOMOD=go mod
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date '+%Y/%m/%d %H:%M:%S')
LDFLAGS=-ldflags="-s -w -X 'github.com/uyloal/baihu-panel/internal/constant.Version=$(VERSION)' -X 'github.com/uyloal/baihu-panel/internal/constant.BuildTime=$(BUILD_TIME)'"

TAGS_WEB=-tags web

.PHONY: all build release release-windows build-web pack-webui clean clean-all run dev deps swag docs-dev docs-build docker-build docker-run docker-up docker-down docker-dev docker-dev-d docker-dev-down docker-dev-clean help

# Default target
all: build

# Build frontend
build-web:
	cd web && pnpm install --frozen-lockfile=false && pnpm run build

pack-webui:
	@echo "==> [1/6] 验证参数有效性..."
	@if [ -z "$(NAME)" ] || [ -z "$(VERSION)" ] || [ -z "$(AUTHOR)" ] || [ -z "$(DESC)" ]; then \
		echo "Error: Missing required arguments!"; \
		echo "Usage: make pack-webui NAME=<name> VERSION=<version> AUTHOR=<author> DESC=<description>"; \
		exit 1; \
	fi
	@if [ "$(NAME)" = "default" ]; then \
		echo "Error: WebUI name cannot be 'default' ('default' is reserved for the built-in system identifier)."; \
		exit 1; \
	fi
	@echo "==> [2/6] 正在安装前端依赖包 (pnpm install)..."
	cd web && pnpm install --frozen-lockfile=false
	@echo "==> [3/6] 正在编译构建前端资源文件 (pnpm run build)..."
	cd web && pnpm run build
	@echo "==> [4/6] 正在准备归档输出目录与清理旧包..."
	@mkdir -p bin
	@rm -f bin/webui-$(NAME)-$(VERSION).tar.gz
	@echo "==> [5/6] 正在生成包配置文件 uimanifest.json..."
	@echo '{"name": "$(NAME)", "version": "$(VERSION)", "author": "$(AUTHOR)", "description": "$(DESC)"}' > web/dist/uimanifest.json
	@echo "==> [6/6] 正在压缩打包为 tar.gz 归档包..."
	@sleep 2
	cd web/dist && tar -czf ../../bin/webui-$(NAME)-$(VERSION).tar.gz *
	@echo "==> 打包成功！资源包已创建于: bin/webui-$(NAME)-$(VERSION).tar.gz"

# Build backend binary (no UI embedded)
build:
	@mkdir -p bin
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BINARY) main.go

# Build full release binary (with UI embedded)
release:
	cd web && pnpm install --frozen-lockfile=false && pnpm run build
	@mkdir -p bin
	rm -rf internal/static/dist
	cp -r web/dist internal/static/dist
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) $(TAGS_WEB) -o $(BINARY) main.go

# Build release version for Windows
release-windows:
	cd web && pnpm run build
	@mkdir -p bin
	rm -rf internal/static/dist
	cp -r web/dist internal/static/dist
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) $(TAGS_WEB) -o bin/baihu.exe main.go

# Alias for backward compatibility
build-all: release

# Clean built files
clean:
	$(GOCLEAN)
	rm -rf bin/
	rm -rf internal/static/dist
	rm -rf web/dist

# Clean everything: local artifacts and Docker development environment (including volumes)
clean-all: clean docker-dev-clean
	rm -rf web/node_modules
	@echo "All local artifacts and Docker dev caches have been completely wiped."

# Run the application
run:
	@mkdir -p bin
	$(GOBUILD) -o $(BINARY) main.go
	./$(BINARY) server

# Development run with hot reload (both frontend and backend)
dev:
	@command -v concurrently > /dev/null 2>&1 || npm install -g concurrently
	@mkdir -p web/node_modules
	concurrently --kill-others \
		"go tool air" \
		"cd web && pnpm install --frozen-lockfile=false && pnpm run dev"

# Install dependencies
deps:
	$(GOMOD) tidy

# Generate swagger documentation
swag:
	@mkdir -p docs/public
	go run github.com/swaggo/swag/cmd/swag@latest init -g main.go -o ./docs/public --ot json,yaml

docs-dev:
	cd docs && npm run docs:dev

docs-build:
	cd docs && npm run docs:build

# Docker build
docker-build:
	docker build -t baihu:latest -f docker/Dockerfile .

# Docker run
docker-run:
	docker run -p 8052:8052 baihu:latest

# Docker compose up
docker-up:
	docker compose up -d

# Docker compose down
docker-down:
	docker compose down

# Start isolated Docker dev environment (foreground with logs, Ctrl+C to stop)
docker-dev:
	@mkdir -p web/node_modules
	docker compose -f docker-compose.dev.yml up --build

# Start isolated Docker dev environment (background)
docker-dev-d:
	docker compose -f docker-compose.dev.yml up -d --build

# Stop Docker dev environment (preserves cached volumes for fast restart)
docker-dev-down:
	docker compose -f docker-compose.dev.yml down

# Stop and completely clean Docker dev environment (removes all cached volumes)
docker-dev-clean:
	docker compose -f docker-compose.dev.yml down -v

# Help
help:
	@echo "Available targets:"
	@echo "  all              - Build backend only (default)"
	@echo "  build            - Build backend binary (no UI embedded)"
	@echo "  release          - Build full release binary (with UI embedded)"
	@echo "  release-windows  - Build full release binary for Windows"
	@echo "  build-web        - Build frontend assets only"
	@echo "  pack-webui       - Build and package custom WebUI tar.gz"
	@echo "  clean            - Clean built files"
	@echo "  clean-all        - Clean local files and Docker dev environment (including volumes)"
	@echo "  run              - Run the application locally"
	@echo "  dev              - Run local development with hot reload"
	@echo "  deps             - Install Go dependencies"
	@echo "  docker-build     - Build production Docker image"
	@echo "  docker-run       - Run production Docker container"
	@echo "  docker-up        - Start production Docker Compose stack"
	@echo "  docker-down      - Stop production Docker Compose stack"
	@echo "  docker-dev       - Start isolated Docker dev environment (foreground)"
	@echo "  docker-dev-d     - Start isolated Docker dev environment (background)"
	@echo "  docker-dev-down  - Stop Docker dev environment (keep caches)"
	@echo "  docker-dev-clean - Stop and clean Docker dev environment (remove caches)"
	@echo "  swag             - Generate swagger documentation and sync with docs"
	@echo "  docs-dev         - Run documentation development server"
	@echo "  docs-build       - Build documentation"
	@echo "  help             - Show this help message"
