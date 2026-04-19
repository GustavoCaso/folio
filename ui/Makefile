.PHONY: build test test-js proto templ

build:
	go build ./...

test:
	@echo "running ui tests"
	@go test ./...

test-js:
	@echo "running ui js tests"
	@mise exec -- pnpm test
	
test-race:
	go test ./... -race

lint:
	golangci-lint run --fix

format:
	golangci-lint fmt .

proto:
	@echo "generating ui proto defintions"
	@mkdir -p internal/parser/proto
	@protoc \
		-I ../proto \
		--go_out=internal/parser/proto --go_opt=paths=source_relative \
		--go-grpc_out=internal/parser/proto --go-grpc_opt=paths=source_relative \
		../proto/parser.proto

templ:
	templ generate
