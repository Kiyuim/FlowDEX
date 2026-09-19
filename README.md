# FlowDEX

A microservice-based decentralized trading platform for Solana, covering the full lifecycle of a trade: on-chain ingestion, price discovery, order execution, and real-time market data delivery.

FlowDEX parses Solana transactions in real time, executes swaps through **Pump.fun** and **Raydium (CPMM/CLMM)**, and streams live trade and K-line data to a React frontend over WebSocket — with MEV-aware execution via **Jito** and protocol-level slippage protection.

## Architecture

![FlowDEX architecture](./framework.png)

Six independent Go services, each with a single responsibility:

| Service | Responsibility |
|---|---|
| **Consumer** | Subscribes to Solana WebSocket/RPC, decodes raw transactions into structured trades, publishes to Kafka |
| **DataFlow** | Consumes the Kafka trade stream, aggregates K-lines (1m/5m/1h/…), persists to MySQL, publishes live updates to Redis pub/sub |
| **Market** | Serves market data (token lists, pair info, K-lines, on-chain security signals) to the frontend |
| **Trade** | Builds and executes swap transactions (Pump.fun / Raydium), handles slippage and MEV protection |
| **Gateway** | Single HTTP entrypoint; routes REST requests to the underlying gRPC services |
| **WebSocket** | Pushes real-time updates (new tokens, K-line ticks) to connected clients |

Services communicate over gRPC (Protobuf), with Kafka decoupling high-throughput on-chain ingestion from downstream aggregation and storage.

## Highlights

- **Async ingestion pipeline** — Solana transactions are parsed once by Consumer and fanned out over Kafka to independent consumer groups (DataFlow), so a burst in on-chain activity doesn't take down downstream processing.
- **Multi-protocol trade execution** — swap construction and execution across Pump.fun and Raydium's CPMM/CLMM pools, sharing one slippage-calculation path.
- **MEV-aware submission** — transactions can route through Jito's private bundle submission instead of the public mempool, with a dynamically-adjusted tip based on the live tip floor, falling back to standard RPC submission when Jito isn't configured.
- **Protocol-level slippage protection** — every swap carries an on-chain minimum-output threshold; if the market moves past the user's tolerance, the transaction reverts instead of executing at a bad price. Auto-slippage retry widens the tolerance in controlled steps after a slippage failure.
- **Wash-trade filtering** — same-block, same-wallet buy+sell pairs are flagged before they reach K-line aggregation, so a self-trading wallet can't put a spike in the candle.
- **Real-time K-line delivery** — DataFlow's aggregated candles are pushed through Redis pub/sub to a WebSocket service, so charts update live without polling.
- **On-chain security signal** — before listing, token mint/freeze authority is read directly from the SPL Token account to flag basic rug-pull risk (revoked mint/freeze authority is a green flag).

## Tech Stack

- **Backend**: Go, [go-zero](https://github.com/zeromicro/go-zero), gRPC, Protobuf
- **Messaging**: Kafka
- **Storage**: MySQL, Redis
- **Chain**: Solana RPC/WebSocket, Jito
- **Frontend**: React

## Getting Started (local)

Spin up MySQL, Redis, and Kafka, seeded with demo data (fictional tokens/pairs/trades — not live chain data):

```bash
cd docker
docker compose up -d
```

Run each service against the local stack:

```bash
cd market    && go run market.go    -f etc/market-local.yaml
cd trade     && go run trade.go     -f etc/trade-local.yaml
cd consumer  && go run consumer.go  -f etc/consumer-local.yaml
cd dataflow  && go run dataflow.go  -f etc/dataflow-local.yaml
cd gateway   && go run gateway.go   -f etc/gateway-local.yaml
cd websocket && go run token_websocket_server.go -f etc/websocket-local.yaml
```

Frontend:

```bash
cd pump-tokens-ui
npm install
npm start
```

Production configs (`etc/*.yaml`, without the `-local` suffix) are environment-specific and are not checked into this repo — copy the `-local.yaml` files and point them at your own MySQL/Redis/Kafka instances to deploy elsewhere. Secrets that must be present (Kafka SASL credentials, RPC API keys, the trade signer's private key) are read from environment variables — see each service's `internal/config/config.go`.

## Project Structure

```
consumer/    # on-chain ingestion + trade decoding
dataflow/    # K-line aggregation + persistence
market/      # market data API
trade/       # swap construction + execution
gateway/     # HTTP entrypoint
websocket/   # real-time push
model/       # shared DB schema + generated models
pkg/         # shared libraries (chain clients, swap math, etc.)
pump-tokens-ui/  # React frontend
docker/      # local dev stack (docker-compose + seed data)
```
