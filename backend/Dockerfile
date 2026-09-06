# iotwong backend image — same image runs server / ingestor / migrate /
# import-racebox via CMD override (docs/06-deployment.md). Runtime user is
# non-root with CA certificates; builds are static (CGO_ENABLED=0).
# Build args for offline-friendly module proxy:
#   docker build --build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=off ...
# NOTE: this file is the compose build file for build context ./backend
# (dockerfile: Dockerfile). Keep in sync with deploy/backend/Dockerfile.
FROM golang:1.27.1-alpine AS builder
ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
ENV GOPROXY=${GOPROXY} GOSUMDB=${GOSUMDB} CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN mkdir -p /out \
 && go build -trimpath -o /out/server        ./cmd/server \
 && go build -trimpath -o /out/ingestor      ./cmd/ingestor \
 && go build -trimpath -o /out/migrate       ./cmd/migrate \
 && go build -trimpath -o /out/import-racebox ./cmd/import-racebox

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S iot && adduser -S -G iot iot
COPY --from=builder /out/ /usr/local/bin/
USER iot
WORKDIR /home/iot
EXPOSE 8080
# The orchestrator overrides the command: e.g. `server`, `ingestor`, `migrate`.
ENTRYPOINT []
CMD ["server"]
