# Minimal Golang image to build and obtain backend binary
FROM golang:1.24-alpine AS backend_builder

# 1. Install git (for downloading go mod dependencies)
RUN apk add --no-cache git

# 2. Install dlv (for debugging)
RUN go install "github.com/go-delve/delve/cmd/dlv@v1.25.1"

WORKDIR /coze-loop

# 3. Download and cache go mod dependencies
COPY ./backend/go.mod ./backend/go.sum /coze-loop/src/backend/
RUN go mod download -C ./src/backend -x

# 4. Build backend binary (with no optimizations, disabled inlining, for debugging)
COPY ./backend/ /coze-loop/src/backend/
RUN mkdir -p ./bin && \
    go -C /coze-loop/src/backend build -gcflags="all=-N -l" -buildvcs=false -o /coze-loop/bin/main "./cmd"

# Final minimal image (coze-loop)
FROM ${COZE_LOOP_APP_IMAGE_REGISTRY:-docker.io}/${COZE_LOOP_APP_IMAGE_REPOSITORY:-cozedev}/${COZE_LOOP_APP_IMAGE_NAME:-coze-loop}:${COZE_LOOP_APP_IMAGE_TAG:-latest}

WORKDIR /coze-loop

# Copy build artifacts
COPY --from=backend_builder /coze-loop/bin/main /coze-loop/bin/main
COPY --from=backend_builder /go/bin/dlv /usr/local/bin/dlv
