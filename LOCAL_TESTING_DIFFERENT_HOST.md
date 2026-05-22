# Testing local builds on separate host

Build a release image locally for `linux/arm64`, then ship it to the Pi in one pipeline — no intermediate tar file needed.

## Build

```bash
# From the repo root
docker buildx build \
  --platform linux/arm64 \
  --target release \
  -t folio-parser:local \
  --load \
  parser/

docker buildx build \
  --platform linux/arm64 \
  -t folio-ui:local \
  --load \
  ui/
```

> `--load` requires a single `--platform`. For multi-platform builds use `--push` to a registry instead.

## Transfer and load in one command

```bash
docker save folio-parser:local | ssh <host> docker load
docker save folio-ui:local     | ssh <host> docker load
```
