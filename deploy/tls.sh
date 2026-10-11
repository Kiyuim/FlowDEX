#!/usr/bin/env bash
# Issue or renew the Let's Encrypt certificate for TLS_HOST (deploy/.env) via
# HTTP-01 on the frontend's :80 webroot, then hot-reload nginx. Safe to run
# repeatedly; cron runs it daily:
#   17 3 * * * /opt/fun_dex_v2/deploy/tls.sh >> /var/log/fundex-tls.log 2>&1
set -euo pipefail

cd "$(dirname "$0")"

set -a
# shellcheck disable=SC1091
. ./.env
set +a

host="${TLS_HOST:-$PUBLIC_HOST}"
profile="${TLS_PROFILE:-}"
email="${ACME_EMAIL:?set ACME_EMAIL in deploy/.env}"
lego_image="goacme/lego:v4.28.1"

mkdir -p certs acme lego

# nginx refuses to start without a cert file, so seed a short-lived
# self-signed one until the first real issuance replaces it.
if [ ! -s certs/fullchain.pem ]; then
  openssl req -x509 -nodes -newkey rsa:2048 -days 2 -subj "/CN=$host" \
    -keyout certs/privkey.pem -out certs/fullchain.pem 2>/dev/null
  docker compose up -d --no-build frontend
fi

args=(--path /data --disable-cn --accept-tos --email "$email" --domains "$host" --http --http.webroot /acme)
if [ -s "lego/certificates/$host.crt" ]; then
  # Renew at a third of the lifetime left: 2 days for 6-day IP certs.
  days=30; [ "$profile" = "shortlived" ] && days=2
  cmd=(renew --days "$days" ${profile:+--profile "$profile"})
else
  cmd=(run ${profile:+--profile "$profile"})
fi

docker run --rm --user "$(id -u):$(id -g)" -v "$PWD/lego:/data" -v "$PWD/acme:/acme" "$lego_image" "${args[@]}" "${cmd[@]}"

if ! cmp -s "lego/certificates/$host.crt" certs/fullchain.pem; then
  cp "lego/certificates/$host.crt" certs/fullchain.pem
  cp "lego/certificates/$host.key" certs/privkey.pem
  chmod 600 certs/privkey.pem
  docker compose exec -T frontend nginx -s reload
  echo "installed new certificate for $host"
fi
