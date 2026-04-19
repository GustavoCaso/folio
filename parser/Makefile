.PHONY: test lint lint-check format typecheck proto

test:
	@echo "running parser tests"
	@uv run pytest tests/

lint:
	uv run ruff check --fix src/ tests/

lint-check:
	uv run ruff check src/ tests/

format:
	uv run ruff format src/ tests/

typecheck:
	uv run mypy src/

proto:
	@echo "generating parser proto defintions"
	@uv run python -m grpc_tools.protoc \
		-I ../proto \
		--python_out=src/parser/grpc \
		--grpc_python_out=src/parser/grpc \
		../proto/parser.proto
	@# Fix generated import
	@sed -i '' 's/import parser_pb2/from parser.grpc import parser_pb2/' src/parser/grpc/parser_pb2_grpc.py
