.PHONY: test lint lint-check format format-check typecheck proto

test:
	@echo "running parser tests"
	@uv run pytest tests/

lint:
	uv run ruff check --fix src/ tests/

lint-check:
	uv run ruff check src/ tests/

format:
	uv run ruff format src/ tests/
	
format-check:
	uv run ruff format --check src/ tests/

typecheck:
	uv run mypy src/

proto:
	@echo "generating parser proto defintions"
	@uv run python -m grpc_tools.protoc \
		-I ../proto \
		--python_out=src/parser/grpc \
		--grpc_python_out=src/parser/grpc \
		../proto/parser.proto
	@uv run protol \
		--in-place \
		--python-out src/parser/grpc \
		--create-package \
		protoc --proto-path=../proto ../proto/parser.proto
