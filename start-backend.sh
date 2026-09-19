#!/bin/sh
# Launch the bundled backend microservices in dependency order.
# market (8080) must be up before trade/consumer, which dial it over localhost.
# gateway (8083) is the public HTTP entrypoint.
set -e

log() { echo "[start-backend] $1"; }

start() {
  name="$1"; dir="$2"; cfg="$3"
  log "starting $name ..."
  ( cd "/app/$dir" && exec /app/bin/"$name" -f "$cfg" ) &
}

start market   market   etc/market.yaml
sleep 4
start trade    trade    etc/trade.yaml
start consumer consumer etc/consumer.yaml
start dataflow dataflow etc/dataflow.yaml
sleep 2
start gateway  gateway  etc/gateway.yaml

log "all services launched; waiting"
# If any background process exits, surface it and stop the container so Railway restarts.
wait -n
code=$?
log "a service exited (code $code); shutting down container"
exit "$code"
