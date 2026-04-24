#!/bin/bash
set -e

SCHEMA_REGISTRY_URL=${SCHEMA_REGISTRY_URL:-http://schema-registry:8081}
PROTO_FILE=${PROTO_FILE:-/proto/movie_event.proto}

echo "Waiting for Schema Registry..."
until curl -sf "$SCHEMA_REGISTRY_URL/subjects" > /dev/null 2>&1; do
  sleep 2
done

SCHEMA_CONTENT=$(python3 -c "
import json, sys
with open('$PROTO_FILE') as f:
    print(json.dumps(f.read()))
" 2>/dev/null || cat "$PROTO_FILE" | sed 's/\\/\\\\/g' | sed 's/"/\\"/g' | awk 'BEGIN{ORS="\\n"}{print}' | sed 's/^/"/;s/$/"/' | tr -d '\n')

curl -s -X POST "$SCHEMA_REGISTRY_URL/subjects/movie-events-value/versions" \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d "{\"schemaType\": \"PROTOBUF\", \"schema\": $SCHEMA_CONTENT}"

echo ""
echo "Schema registered. Versions:"
curl -s "$SCHEMA_REGISTRY_URL/subjects/movie-events-value/versions"
echo ""
