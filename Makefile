.PHONY: help backend frontend build dev test clean

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

backend: ## 构建后端单二进制 → bin/niuniu-api
	cd backend && CGO_ENABLED=0 go build -o ../bin/niuniu-api ./cmd/niuniu

frontend: ## 构建前端 → frontend/dist
	cd frontend && npm run build

build: backend frontend ## 同时构建前后端

dev: ## 启动开发模式（后端 + 前端，需手动分两个终端，或用 dev-backend / dev-frontend）
	@echo "请在两个终端分别执行：make dev-backend 与 make dev-frontend"

dev-backend: ## 启动后端（热重载用 air 或 go run）
	cd backend && go run ./cmd/niuniu -port 8787

dev-frontend: ## 启动前端 vite dev server (5173，proxy 到后端 8787)
	cd frontend && npm run dev

run: ## 用构建产物启动后端（前端需另起静态服务或 vite dev）
	./bin/niuniu-api -port 8787

test: ## 运行测试（后端 go test + 前端 vitest）
	cd backend && go test ./...
	cd frontend && npm run test

test-backend: ## 仅后端测试
	cd backend && go test ./...

test-frontend: ## 仅前端测试
	cd frontend && npm run test

test-e2e: ## 端到端测试（Playwright，自动启动前后端）
	cd e2e && npm install && PLAYWRIGHT_HTML_OPEN=never npx playwright test

clean: ## 清理构建产物与数据目录
	rm -rf bin frontend/dist
	rm -rf data/niuniu.db data/niuniu.db-* data/config.yaml
