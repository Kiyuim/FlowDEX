#!/usr/bin/env bash
# Render deploy/templates/*.yaml -> deploy/config/*.yaml, substituting the
# variables from deploy/.env. Run this after ANY change to .env or templates,
# then `docker compose up -d` (the configs are bind-mounted, so most changes
# only need a service restart, not a rebuild).
set -euo pipefail

cd "$(dirname "$0")"

[ -f .env ] || { echo "deploy/.env not found — copy .env.example to .env first" >&2; exit 1; }
command -v envsubst >/dev/null || { echo "envsubst missing — apt-get install -y gettext-base" >&2; exit 1; }

set -a
# shellcheck disable=SC1091
. ./.env
set +a

case "${SOL_NODE_URL:-}" in
  *helius-rpc.com*api-key=) echo "HELIUS_API_KEY is empty in deploy/.env" >&2; exit 1 ;;
esac

mkdir -p config
for tpl in templates/*.yaml; do
  out="config/$(basename "$tpl")"
  envsubst < "$tpl" > "$out"
  echo "rendered $out"
done

# Fail loudly instead of shipping a config with an unsubstituted placeholder.
if grep -rn '\${' config/ >/dev/null 2>&1; then
  echo "ERROR: unsubstituted variables remain:" >&2
  grep -rn '\${' config/ >&2
  exit 1
fi
