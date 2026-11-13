GO ?= go
NPM ?= npm
SERVER_DIR := apps/server
WEB_DIR := apps/midway

.PHONY: help install dev dev-server dev-web test test-server test-web build build-server build-web lint typecheck clean

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

install: ## Instala dependencias del frontend
	$(NPM) install

dev: ## Levanta backend y frontend en paralelo
	@echo "Backend  -> http://localhost:8080"
	@echo "Frontend -> http://localhost:5173"
	@($(MAKE) dev-server & $(MAKE) dev-web & wait)

dev-server: ## Levanta solo el backend Go
	cd $(SERVER_DIR) && $(GO) run ./cmd/midway

dev-web: ## Levanta solo la SPA de Vue
	$(NPM) --workspace $(WEB_DIR) run dev

test: test-server test-web ## Ejecuta todas las pruebas

test-server: ## Pruebas del backend Go
	cd $(SERVER_DIR) && $(GO) test ./...

test-web: ## Pruebas del frontend
	$(NPM) --workspace $(WEB_DIR) run test

build: build-server build-web ## Compila backend y frontend

build-server: ## Compila el binario del backend
	cd $(SERVER_DIR) && $(GO) build -o bin/midway ./cmd/midway

build-web: ## Build de producción del frontend
	$(NPM) --workspace $(WEB_DIR) run build

lint: ## Lint del frontend
	$(NPM) --workspace $(WEB_DIR) run lint

typecheck: ## Chequeo de tipos del frontend
	$(NPM) --workspace $(WEB_DIR) run typecheck

clean: ## Limpia artefactos de build
	rm -rf $(SERVER_DIR)/bin $(WEB_DIR)/dist
