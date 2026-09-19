# syntax=docker/dockerfile:1
#
# Backend image for Railway. Builds all Go microservices and runs them together
# in one container so their localhost gRPC wiring keeps working unchanged.
#
# The `backend` Railway service uses the default CMD (start-backend.sh).
# The `websocket` Railway service reuses THIS SAME image but overrides the start
# command with:  cd /app/websocket && /app/bin/websocket -f etc/websocket.yaml
#
# ---- build stage ----
FROM golang:1.24.2 AS builder
ENV GOPROXY=https://proxy.golang.org,direct
ENV CGO_ENABLED=0 GOOS=linux
WORKDIR /src

# Cache root-module deps first
COPY go.mod go.sum ./
# vendor-patches must exist before `go mod download` — go.mod's replace
# directive points at it locally, not a fetchable module.
COPY vendor-patches ./vendor-patches
RUN go mod download

COPY . .

# Root module `dex` services. NOTE: market/ has several `package main` files, so we
# build from the specific entrypoint file (file-list form) to avoid duplicate main().
RUN mkdir -p /out && \
    go build -ldflags="-s -w" -o /out/market    ./market/market.go && \
    go build -ldflags="-s -w" -o /out/trade     ./trade/trade.go && \
    go build -ldflags="-s -w" -o /out/consumer  ./consumer/consumer.go && \
    go build -ldflags="-s -w" -o /out/dataflow  ./dataflow/dataflow.go && \
    go build -ldflags="-s -w" -o /out/gateway   ./gateway/gateway.go

# websocket/ is its OWN Go module (module `websocket`) and must build from its own dir.
RUN cd websocket && go mod download && \
    go build -ldflags="-s -w" -o /out/websocket ./token_websocket_server.go

# ---- runtime stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates
WORKDIR /app

COPY --from=builder /out/ /app/bin/

# Each service reads its config relative to its own working dir (etc/<svc>.yaml)
COPY --from=builder /src/market/etc    /app/market/etc
COPY --from=builder /src/trade/etc     /app/trade/etc
COPY --from=builder /src/consumer/etc  /app/consumer/etc
COPY --from=builder /src/dataflow/etc  /app/dataflow/etc
COPY --from=builder /src/gateway/etc   /app/gateway/etc
COPY --from=builder /src/websocket/etc /app/websocket/etc

COPY start-backend.sh /app/start-backend.sh
RUN chmod +x /app/start-backend.sh

# gateway HTTP port (set Railway's target port to 8083 for the backend service)
EXPOSE 8083
# websocket port (target port 8086 for the websocket service)
EXPOSE 8086

CMD ["/app/start-backend.sh"]
