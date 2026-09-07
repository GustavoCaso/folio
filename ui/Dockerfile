FROM golang:1.27-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o folio-ui .

# Debian (glibc), not alpine: the AI_PIPELINE_ENABLED cleanup pass loads
# Kronk's precompiled llama.cpp/ggml shared libraries at runtime
# (github.com/ardanlabs/kronk), which are only published for glibc-based
# Linux (amd64/arm64, CPU/CUDA/Vulkan) — see
# github.com/ardanlabs/kronk/sdk/tools/libs.SupportedCombinations. musl/alpine
# is not part of that build matrix. libgomp1 is required too: the CPU-backend
# tarball links libgomp.so.1 (OpenMP), which debian-slim does not ship by
# default — verified against the real llama-b10638-bin-ubuntu-cpu-arm64
# tarball; without it, kronk.Init fails with "libgomp.so.1: cannot open
# shared object file".
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates tzdata gosu passwd libgomp1 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r folio && useradd -r -g folio -d /data/kronk-home folio

WORKDIR /app
COPY --from=builder /app/folio-ui .
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh && mkdir -p /data && chown folio:folio /data

ENV DB_PATH=/data/folio.db \
    PARSER_GRPC_ADDR=localhost:50051 \
    DATA_DIR=/data \
    LOG_LEVEL=info

# Kronk downloads its model (AI_PIPELINE_MODEL) and llama.cpp/ggml libraries
# into $HOME/.kronk on first use. Mount this as
# a volume in production so the ~4GB+ model download persists across
# container restarts instead of re-downloading every time.
#
# HOME is NOT set via ENV here: entrypoint.sh execs the app via `gosu folio`,
# and gosu always resets HOME from the target user's /etc/passwd entry,
# discarding any inherited HOME env var (verified — an ENV HOME=... override
# here is silently clobbered at runtime). So folio's home directory itself is
# set to /data/kronk-home via `useradd -d` above instead.
RUN mkdir -p /data/kronk-home && chown folio:folio /data/kronk-home

VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
CMD ["./folio-ui"]
