.PHONY: proto build up down logs logs-all logs-ch logs-aggregator logs-grafana logs-minio test-event test-integration test-integration-all clean check-health check-kafka check-ch check-ch-sample check-ch-tables check-pg check-pg-sample run-agg run-export check-grafana check-s3

proto:
	protoc \
		--go_out=producer/pb \
		--go_opt=paths=source_relative \
		--proto_path=proto \
		proto/movie_event.proto

build:
	docker-compose build

up:
	docker-compose up -d

down:
	docker-compose down -v

clean:
	docker-compose down -v --rmi local

logs:
	docker-compose logs -f producer

logs-all:
	docker-compose logs -f

logs-ch:
	docker-compose logs -f clickhouse clickhouse-init

logs-aggregator:
	docker-compose logs -f aggregator

logs-grafana:
	docker-compose logs -f grafana

logs-minio:
	docker-compose logs -f minio minio-init

test-event:
	curl -s -X POST http://localhost:8080/api/v1/events \
		-H "Content-Type: application/json" \
		-d '{"event_id":"550e8400-e29b-41d4-a716-446655440000","user_id":"user-0001","movie_id":"movie-0001","event_type":"VIEW_STARTED","device_type":"MOBILE","session_id":"sess-001","progress_seconds":0}' | jq .

test-integration:
	cd tests && go test -v -timeout 300s ./integration/ -count=1

test-integration-all: up
	@echo "Waiting for services to be ready..."
	@sleep 30
	$(MAKE) test-integration

check-ch:
	curl -s -u $${CLICKHOUSE_USER:-app}:$${CLICKHOUSE_PASSWORD:-app} 'http://localhost:8123/' --data-binary "SELECT count() FROM movie_events"

check-ch-sample:
	curl -s -u $${CLICKHOUSE_USER:-app}:$${CLICKHOUSE_PASSWORD:-app} 'http://localhost:8123/' --data-binary "SELECT * FROM movie_events LIMIT 5 FORMAT Pretty"

check-ch-tables:
	curl -s -u $${CLICKHOUSE_USER:-app}:$${CLICKHOUSE_PASSWORD:-app} 'http://localhost:8123/' --data-binary "SHOW TABLES FORMAT Pretty"

check-health:
	curl -s http://localhost:8080/health | jq .
	curl -s http://localhost:8080/ready | jq .
	curl -s http://localhost:8090/health | jq .
	curl -s http://localhost:3000/api/health | jq .

check-kafka:
	docker-compose exec -T kafka1 kafka-topics --bootstrap-server kafka1:29092,kafka2:29092 --describe --topic movie-events
	docker-compose exec -T kafka1 kafka-configs --bootstrap-server kafka1:29092,kafka2:29092 --entity-type topics --entity-name movie-events --describe | grep min.insync.replicas
	curl -sf http://localhost:8081/subjects | jq .

check-grafana:
	curl -s http://localhost:3000/api/health | jq .

check-pg:
	docker-compose exec -T postgres psql -U app -d analytics -c "SELECT count(*) AS rows FROM business_metrics;"

check-pg-sample:
	docker-compose exec -T postgres psql -U app -d analytics -c "SELECT * FROM business_metrics ORDER BY computed_at DESC LIMIT 20;"

run-agg:
	curl -s -X POST "http://localhost:8090/aggregate/run?date=$${DATE:-$$(date -u +%F)}" | jq .

run-export:
	curl -s -X POST "http://localhost:8090/export/run?date=$${DATE:-$$(date -u +%F)}" | jq .

check-s3:
	docker-compose run --rm --no-deps --entrypoint /bin/sh minio-init -c "mc alias set local http://minio:9000 minioadmin minioadmin >/dev/null && mc ls --recursive local/movie-analytics"

