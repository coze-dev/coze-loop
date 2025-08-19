# Minimal Golang image to build and obtain backend binary
FROM golang:1.24-alpine AS backend_builder

# 1. Install git (for downloading go mod dependencies)
RUN apk add --no-cache git

WORKDIR /coze-loop

# 2. Download and cache go mod dependencies
COPY ./backend/go.mod ./backend/go.sum /coze-loop/src/backend/
RUN go mod download -C ./src/backend -x

# 3. Build backend binary
COPY ./backend/ /coze-loop/src/backend/
RUN mkdir -p ./bin && \
    go -C /coze-loop/src/backend build -buildvcs=false -o /coze-loop/bin/main "./cmd"

# Minimal Node.js image (with Node.js + npm), additionally installs Rush to build frontend artifacts
FROM node:20.13.1-alpine AS frontend_builder

# 1. Install basic tools (curl, bash, etc.) for alpine
RUN apk add --no-cache bash

# 2. Install pnpm and Rush
RUN corepack enable && \
    corepack prepare pnpm@8.15.8 --activate && \
    npm install -g @microsoft/rush@5.147.1

WORKDIR /coze-loop

# 3. Build frontend
COPY ./frontend/ /coze-loop/src/frontend/
COPY ./common/ /coze-loop/src/common/
COPY ./rush.json /coze-loop/src/rush.json
RUN mkdir -p /coze-loop/resources && \
    sh /coze-loop/src/frontend/apps/cozeloop/build-artifact.sh /coze-loop/resources

# Final minimal image (coze-loop)
FROM alpine:3.22.0

WORKDIR /coze-loop

# Extract build artifacts
COPY --from=backend_builder /coze-loop/bin/main /coze-loop/bin/main
COPY --from=frontend_builder /coze-loop/resources/ /coze-loop/resources/