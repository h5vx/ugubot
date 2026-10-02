#!/bin/sh
# End-to-end test: Prosody + Postgres + ugubot (all services, embedded NATS)
# + a driver that plays XMPP users, the web client and a fake OpenAI.
# Requires docker. Usage: [MODE=split] test/e2e/run.sh [keep]
#   MODE=all   (default) one process with embedded NATS
#   MODE=split every service in its own container with an external NATS
set -eu
MODE=${MODE:-all}
containers="ugu-prosody ugu-pg ugu-nats ugu-history ugu-xmpp ugu-ai ugu-bot ugu-e2e"
cd "$(dirname "$0")/../.."

work=$(mktemp -d)
trap 'rm -rf "$work"; [ "${1:-}" = keep ] || docker rm -f $containers >/dev/null 2>&1 || true' EXIT

CGO_ENABLED=0 go build -o "$work/ugubot" ./cmd/ugubot
CGO_ENABLED=0 go build -o "$work/e2e" ./test/e2e
cp test/e2e/settings.toml "$work/"
mkdir "$work/certs"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,DNS:conference.localhost" \
  -keyout "$work/certs/localhost.key" -out "$work/certs/localhost.crt" 2>/dev/null

docker rm -f $containers >/dev/null 2>&1 || true
docker network create ugunet >/dev/null 2>&1 || true

docker run -d --name ugu-pg --network ugunet -e POSTGRES_PASSWORD=x -e POSTGRES_DB=ugubot postgres:17-alpine >/dev/null

docker create --name ugu-prosody --network ugunet prosodyim/prosody:13.0 >/dev/null
tar -C test/e2e --mode=a+rX -cf - prosody.cfg.lua | docker cp - ugu-prosody:/etc/prosody/
tar -C "$work" --mode=a+rX -cf - certs | docker cp - ugu-prosody:/
docker start ugu-prosody >/dev/null
sleep 3
for u in bot alice bob; do docker exec ugu-prosody prosodyctl register $u localhost ${u}pass; done

# bot <container> <service> [docker run options]
bot() {
  name=$1 service=$2
  shift 2
  docker create --name "$name" --network ugunet -w /app "$@" alpine /app/ugubot "$service" >/dev/null
  tar -C "$work" -cf - ugubot settings.toml | docker cp - "$name":/app/
  docker start "$name" >/dev/null
}

if [ "$MODE" = split ]; then
  docker run -d --name ugu-nats --network ugunet nats:2.11-alpine --jetstream >/dev/null
  nats="-e UGUBOT_NATS__URL=nats://ugu-nats:4222"
  bot ugu-history history $nats
  bot ugu-xmpp xmpp-gateway $nats
  bot ugu-ai ai-worker $nats
  bot ugu-bot web-gateway $nats
else
  bot ugu-bot all
fi

docker create --name ugu-e2e --network ugunet alpine /e2e >/dev/null
tar -C "$work" -cf - e2e | docker cp - ugu-e2e:/
docker start -a ugu-e2e || {
  for c in ugu-history ugu-xmpp ugu-ai ugu-bot; do
    docker inspect "$c" >/dev/null 2>&1 && { echo "--- $c logs"; docker logs "$c" 2>&1 | tail -40; }
  done
  exit 1
}
