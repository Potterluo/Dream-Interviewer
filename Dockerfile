# --- Stage 1: Build web UI (static export) ---------------------------------
FROM node:22-alpine AS web-builder
WORKDIR /src/web
# Pin pnpm so lockfile + build behavior stay reproducible.
RUN corepack enable && corepack prepare pnpm@10.15.0 --activate
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# --- Stage 2: Build the Go binary with the UI embedded ----------------------
FROM golang:1.25-alpine AS go-builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /src/web/out internal/server/dist
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
# -trimpath: without it the binary embeds the absolute paths of every source
# file and module-cache entry (e.g. /home/<user>/go/pkg/mod/...), which leaks
# the builder's username into a published image and breaks reproducibility.
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w \
      -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE} \
      -X github.com/Potterluo/dream-interviewer/internal/buildinfo.Version=${VERSION} \
      -X github.com/Potterluo/dream-interviewer/internal/buildinfo.Commit=${COMMIT} \
      -X github.com/Potterluo/dream-interviewer/internal/buildinfo.Date=${DATE}" \
    -o /app ./cmd/server

# --- Stage 3: Minimal runtime -----------------------------------------------
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=go-builder /app /usr/local/bin/app

# Data directory for the SQLite database (override with APP_DATA_DIR).
ENV APP_DATA_DIR=/data
RUN mkdir -p /data
VOLUME /data

EXPOSE 8080
ENTRYPOINT ["app"]
