# syntax=docker/dockerfile:1.7

# Python FaaS Pyodide image built via BuildKit multi-context
# Files are copied from named build context: --build-context bootstrap=release/deployment/docker-compose/bootstrap/python-faas

FROM denoland/deno:1.45.5

# Environment variables
ENV DENO_DIR=/tmp/faas-workspace/.deno \
    DENO_NO_UPDATE_CHECK=1 \
    FAAS_WORKSPACE=/tmp/faas-workspace \
    FAAS_PORT=8000 \
    FAAS_TIMEOUT=30000 \
    FAAS_LANGUAGE=python \
    PYODIDE_VERSION=0.26.2 \
    FAAS_POOL_MIN_SIZE=2 \
    FAAS_POOL_MAX_SIZE=8 \
    FAAS_POOL_IDLE_TIMEOUT=300000 \
    FAAS_MAX_EXECUTION_TIME=30000 \
    FAAS_PRELOAD_TIMEOUT=60000

# Install system deps
USER root
RUN apt-get update && apt-get install -y \
    curl \
    unzip \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Pre-vendor remote deps into image (scripts will be mounted at runtime)
RUN mkdir -p /tmp/faas-workspace/vendor && \
    deno vendor jsr:@eyurtsev/pyodide-sandbox@0.0.3 --output=/tmp/faas-workspace/vendor && \
    echo '{"imports":{"https://jsr.io/":"./jsr.io/"},"scopes":{"./jsr.io/":{"jsr:@eyurtsev/pyodide-sandbox@0.0.3":"./jsr.io/@eyurtsev/pyodide-sandbox/0.0.3/main.ts","jsr:@std/path@^1.0.8":"./jsr.io/@std/path/1.1.2/mod.ts","jsr:/@std/cli@^1.0.16/parse-args":"./jsr.io/@std/cli/1.0.23/parse_args.ts","jsr:@std/internal@^1.0.10/os":"./jsr.io/@std/internal/1.0.12/os.ts"}}}' > /tmp/faas-workspace/vendor/import_map.json && \
    mkdir -p /app/vendor && \
    cp -r /tmp/faas-workspace/vendor/* /app/vendor/

# Non-root user
RUN groupadd -r faas && useradd -r -g faas faas && \
    mkdir -p /tmp/faas-workspace && \
    chown -R faas:faas /app /tmp/faas-workspace

USER faas

## Healthcheck is defined in docker-compose to use mounted script


