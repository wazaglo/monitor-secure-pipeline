.PHONY: help up down restart logs build clean lint test docs

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

up: ## Start all services
	docker compose up -d

down: ## Stop all services
	docker compose down

restart: ## Restart all services
	docker compose restart

logs: ## Tail all logs
	docker compose logs -f

build: ## Rebuild all images
	docker compose build

clean: ## Stop and remove all containers, volumes, images
	docker compose down -v --rmi all

lint: ## Lint Go, Python, and config files
	@echo "--- docker-compose config ---"
	docker compose config -q
	@echo "--- prometheus rules ---"
	docker run --rm -v $(PWD)/monitoring/prometheus:/etc/prometheus prom/prometheus:v2.53.5 promtool check rules /etc/prometheus/rules.yml
	@echo "--- prometheus config ---"
	docker run --rm -v $(PWD)/monitoring/prometheus:/etc/prometheus prom/prometheus:v2.53.5 promtool check config /etc/prometheus/prometheus.yml
	@echo "--- otelcol config ---"
	docker run --rm -v $(PWD)/monitoring/otelcol/config.yml:/etc/otelcol-contrib/config.yaml otel/opentelemetry-collector-contrib:0.158.0 validate --config /etc/otelcol-contrib/config.yaml
	@echo "--- python syntax ---"
	python3 -m py_compile load-generator/main.py
	python3 -m py_compile monitoring/exporters/defectdojo-exporter/main.py
	@echo "All checks passed"

test: ## Run tests (placeholder)
	@echo "No tests configured yet"

docs: ## Serve docs locally
	cd docs && mkdocs serve

docs-build: ## Build static docs site
	cd docs && mkdocs build

status: ## Show service status
	docker compose ps
