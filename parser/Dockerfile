FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim AS builder

WORKDIR /app

COPY pyproject.toml uv.lock ./
# Install dependencies only — cached independently from source changes
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --no-dev --no-install-project

COPY src/ src/
# Install the project itself now that source is present
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --no-dev
    
RUN --mount=type=cache,target=/root/.cache/uv \
    uv run docling-tools models download -o /app/models

FROM python:3.12-slim-bookworm

RUN apt-get update && apt-get install -y curl libgl1-mesa-dev libglib2.0-0 gosu passwd \
    && curl -fsSL https://deb.nodesource.com/setup_24.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r folio && useradd -r -g folio folio

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

COPY --from=builder /app/.venv ./.venv
COPY --from=builder /app/src ./src
COPY --from=builder /app/models ./models
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh && chown -R folio:folio /app

ENV PYTHONPATH=/app/src:$PYTHONPATH
ENV PATH=/app/node_modules/.bin:$PATH

ENV PATH="/app/.venv/bin:$PATH" \
    DOCLING_ARTIFACTS_PATH=/app/models \
    GRPC_PORT=50051 \
    NUM_WORKERS=2 \
    PDF_GENERATE_IMAGES=false \
    PDF_DO_OCR=true \
    PDF_DO_TABLE_STRUCTURE=true \
    PDF_DO_CODE_ENRICHMENT=false \
    PDF_POST_PROCESS_CODE_BLOCKS=true \
    PDF_IMAGE_MODE=placeholder \
    LOG_LEVEL=info

EXPOSE 50051
ENTRYPOINT ["./entrypoint.sh"]
CMD ["parser-server"]
