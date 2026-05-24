.PHONY: proto test up down lint-check format-check proto-check templ-check css-check

proto:
	@cd ui && make proto
	@cd parser && make proto

test:
	@cd ui && make test
	@cd ui && make test-js
	@cd parser && make test
	
up: proto
	@cd ui && make generate
	@cd parser && make download-models
	@docker compose -p folio up --build -d 
	
down:
	@docker compose -p folio down

lint-check:
	@cd ui && make lint-check
	@cd parser && make lint-check

format-check:
	@cd ui && make format-check
	@cd parser && make format-check

proto-check:
	@make proto
	@git diff --exit-code ui/internal/parser/proto/ parser/src/parser/grpc/ || (echo "proto files are out of date; run: make proto" && exit 1)

templ-check:
	@cd ui && make templ
	@git diff --exit-code -- '*_templ.go' || (echo "templ generated files are out of date; run: cd ui && make templ" && exit 1)

css-check:
	@cd ui && make css
	@git diff --exit-code ui/internal/handlers/static/tailwind/output.css || (echo "tailwind output is out of date; run: cd ui && make css" && exit 1)
