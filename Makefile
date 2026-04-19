.PHONY: proto

proto:
	@cd ui && make proto
	@cd parser && make proto

test:
	@cd ui && make test
	@cd ui && make test-js
	@cd parser && make test
	
up:
	@docker compose -p folio up --build -d 
	
down:
	@docker compose -p folio down
