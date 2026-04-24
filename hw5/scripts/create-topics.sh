#!/bin/bash
set -e

KAFKA_BROKER=${KAFKA_BROKER:-kafka1:29092,kafka2:29092}

echo "Waiting for Kafka..."
until kafka-topics --bootstrap-server "$KAFKA_BROKER" --list > /dev/null 2>&1; do
  sleep 2
done

kafka-topics --create \
  --bootstrap-server "$KAFKA_BROKER" \
  --topic movie-events \
  --partitions 3 \
  --replication-factor 2 \
  --if-not-exists \
  --config min.insync.replicas=1 \
  --config retention.ms=604800000

echo "Topic created:"
kafka-topics --describe --bootstrap-server "$KAFKA_BROKER" --topic movie-events
