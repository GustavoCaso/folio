.PHONY: build test test-js test-race lint lint-check format format-check templ css generate

build:
	go build ./...

test:
	@echo "running tests"
	@go test ./...

test-js:
	@echo "running js tests"
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
		
generate:
	@make templ
	@make css

templ:
	templ generate

css:
	@TEMPLUI_PATH="$$(go list -mod=mod -m -f '{{.Dir}}' github.com/templui/templui)" && \
	printf '%s\n' '@source "./**/*.templ";' "@source \"$$TEMPLUI_PATH/components/**/*.templ\";" \
		> internal/handlers/static/tailwind/sources.generated.css
	@mise exec -- npm exec tailwindcss -- \
		-i internal/handlers/static/tailwind/input.css \
		-o internal/handlers/static/tailwind/output.css

templ-check:
	@make templ
	@git diff --exit-code -- '*_templ.go' || (echo "templ generated files are out of date; run: make templ" && exit 1)

css-check:
	@make css
	@git diff --exit-code internal/handlers/static/tailwind/output.css || (echo "tailwind output is out of date; run: make css" && exit 1)
