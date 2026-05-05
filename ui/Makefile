.PHONY: build test test-js test-race lint lint-check format format-check proto templ css generate

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

lint-check:
	golangci-lint run

format:
	golangci-lint fmt .

format-check:
	golangci-lint fmt --diff .

proto:
	@echo "generating ui proto defintions"
	@mkdir -p internal/parser/proto
	@protoc \
		-I ../proto \
		--go_out=internal/parser/proto --go_opt=paths=source_relative \
		--go-grpc_out=internal/parser/proto --go-grpc_opt=paths=source_relative \
		../proto/parser.proto
		
generate:
	make templ
	make css

templ:
	templ generate

css:
	@TEMPLUI_PATH="$$(go list -mod=mod -m -f '{{.Dir}}' github.com/templui/templui)" && \
	printf '%s\n' '@source "./**/*.templ";' "@source \"$$TEMPLUI_PATH/components/**/*.templ\";" \
		> internal/handlers/static/tailwind/sources.generated.css
	@mise exec -- pnpm exec tailwindcss \
		-i internal/handlers/static/tailwind/input.css \
		-o internal/handlers/static/tailwind/output.css
