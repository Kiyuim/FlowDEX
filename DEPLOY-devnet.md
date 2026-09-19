# Deploying the `devnet` branch (Railway + Vercel)

This deploys the **backend microservices to Railway** and the **React frontend to Vercel**.
MySQL (Tencent CDB), Kafka, and Redis (Aliyun) are already cloud-hosted and are reused as-is.

## Topology

```
            Vercel (static React)
                   │
   /direct-api/* , /v1/*   (vercel.json rewrites)
                   │
                   ▼
   Railway: "backend" service  ──────────────►  Tencent MySQL, remote Kafka, Aliyun Redis
   (one container: gateway:8083 + trade:8081 +
    market:8080 + consumer:8088 + dataflow:8084)
                   ▲
   wss:// (REACT_APP_WS_URL)
                   │
   Railway: "websocket" service (websocket:8086)  ──►  Aliyun Redis (pub/sub)
```

All five backend services run in ONE container so their `localhost` gRPC wiring keeps working.
The websocket service runs from the **same image** but with an overridden start command (Railway
exposes only one public port per service, so it needs its own service for its own `wss://` domain).

## What was changed in the repo (already committed-ready)

- `Dockerfile` — builds all 6 Go binaries, runs the backend bundle via `start-backend.sh`.
- `start-backend.sh` — launches market → trade → consumer → dataflow → gateway in order.
- `.dockerignore` — keeps the build context small.
- Config fixes (in place):
  - `market/etc/market.yaml`: `ListenOn` 8089 → **8080** (everyone dials market at 8080).
  - `market`, `trade`, `consumer` Redis → unified to the **Aliyun** Redis (so `consumer`→`websocket`
    pub/sub shares one Redis).
  - `dataflow/etc/dataflow.yaml`: MySQL `localhost` → **Tencent CDB**.
- Frontend: hardcoded `ws://118.194.235.63:80xx` replaced with env-driven `REACT_APP_WS_URL`.
- `pump-tokens-ui/vercel.json` — rewrites `/direct-api/*` and `/v1/*` to the Railway backend.

---

## Step 1 — Railway: `backend` service

1. Push the `devnet` branch to GitHub.
2. Railway → **New Project → Deploy from GitHub repo** → pick the repo, branch `devnet`.
3. It auto-detects the root `Dockerfile`. Build it.
4. **Settings → Networking → Generate Domain.** Set the **target port to `8083`**.
5. Deploy. Note the public URL, e.g. `https://backend-xxxx.up.railway.app`.

> Egress note: the Aliyun Redis and Tencent MySQL must accept connections from Railway's IPs.
> If they're IP-whitelisted, add Railway's egress range (or temporarily open them) or they'll time out.

## Step 2 — Railway: `websocket` service

1. In the **same project**: **New → GitHub Repo → same repo/branch** (creates a 2nd service on the same image).
2. **Settings → Deploy → Custom Start Command:**
   ```
   sh -c 'cd /app/websocket && exec /app/bin/websocket -f etc/websocket.yaml'
   ```
3. **Settings → Networking → Generate Domain.** Set the **target port to `8086`**.
4. Deploy. Note the URL, e.g. `https://websocket-xxxx.up.railway.app`.

## Step 3 — Vercel: frontend

1. Edit `pump-tokens-ui/vercel.json`: replace **both** `REPLACE_BACKEND_URL` with the
   Step-1 backend host (no scheme), e.g. `backend-xxxx.up.railway.app`. Commit.
2. Vercel → **New Project → import repo**. Set **Root Directory = `pump-tokens-ui`**
   (framework preset: Create React App; build `npm run build`, output `build`).
3. **Environment Variables:**
   | Name | Value |
   |---|---|
   | `REACT_APP_WS_URL` | `wss://websocket-xxxx.up.railway.app` (Step-2 host) |
   | `REACT_APP_HELIUS_API_KEY` | your Helius devnet key (optional; default is in code) |
4. Deploy. Open the Vercel URL.

## Verify

- `https://backend-xxxx.up.railway.app/v1/market/index_pump?chain_id=100000&pump_status=1&page_no=1&page_size=5`
  should return JSON.
- Frontend token list loads, and the WS status shows connected (token feed on `/ws/tokens`).

## Known gaps

- **Kline chart (`/ws/kline`, legacy port 8085):** no backend serves this. The chart's live updates
  won't connect until a kline WS service exists. Set `REACT_APP_KLINE_WS_URL` once it does.
- Secrets (DB/Redis/Kafka passwords, Helius key) are currently in the committed YAML. For anything
  beyond devnet, move them to Railway env vars / go-zero env overrides.
