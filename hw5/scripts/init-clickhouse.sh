#!/bin/bash
set -e

CLICKHOUSE_HOST=${CLICKHOUSE_HOST:-clickhouse}
CLICKHOUSE_PORT=${CLICKHOUSE_PORT:-8123}
CLICKHOUSE_USER=${CLICKHOUSE_USER:-app}
CLICKHOUSE_PASSWORD=${CLICKHOUSE_PASSWORD:-app}

auth_url="http://${CLICKHOUSE_USER}:${CLICKHOUSE_PASSWORD}@${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/"

echo "Waiting for ClickHouse..."
until curl -sf "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/ping" > /dev/null 2>&1; do
  sleep 2
done
echo "ClickHouse is ready"

for migration in /migrations/clickhouse/*.sql; do
  echo "Applying: $(basename $migration)"
  curl -fsS "$auth_url" \
    --data-binary @"$migration"
  echo " -> done"
done

echo "All ClickHouse migrations applied"

echo "Verifying tables:"
curl -fsS "$auth_url" \
  --data-binary "SHOW TABLES FROM default FORMAT Pretty"
