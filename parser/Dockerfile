FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim AS builder

WORKDIR /app

COPY pyproject.toml uv.lock ./
RUN uv sync --no-dev

COPY src/ src/

FROM python:3.12-slim-bookworm

RUN apt-get update && apt-get install -y libgl1-mesa-dev libglib2.0-0 nodejs npm && rm -rf /var/lib/apt/lists/* \
    && npm install -g prettier@3.8.3

WORKDIR /app
COPY --from=builder /app/.venv ./.venv
COPY --from=builder /app/src ./src

ENV PYTHONPATH=/app/src:$PYTHONPATH

EXPOSE 50051
CMD [".venv/bin/parser-server"]
