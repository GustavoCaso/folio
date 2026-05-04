FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o folio-ui .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata su-exec shadow && \
    addgroup -S folio && adduser -S folio -G folio
WORKDIR /app
COPY --from=builder /app/folio-ui .
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh && mkdir -p /data && chown folio:folio /data

ENV DB_PATH=/data/folio.db \
    PARSER_GRPC_ADDR=localhost:50051 \
    DATA_DIR=/data \
    LOG_LEVEL=info

VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
CMD ["./folio-ui"]
