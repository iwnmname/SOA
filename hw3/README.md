## Структура проекта

```text
hw3/
├── booking-service/
│   ├── cmd/
│   │   └── main.go                                # Точка входа Booking Service: env, БД, миграции, gRPC клиент, HTTP сервер
│   ├── internal/
│   │   ├── grpcauth/
│   │   │   └── client_interceptor.go             # gRPC client interceptor: добавляет x-api-key в metadata
│   │   ├── handler/
│   │   │   └── handler.go                        # REST endpoints /flights и /bookings, оркестрация GetFlight/ReserveSeats/ReleaseReservation
│   │   ├── model/
│   │   │   └── booking.go                        # Модели Booking и CreateBookingRequest
│   │   └── repository/
│   │       └── booking.go                        # SQL-операции для bookings (create/get/list/update status)
│   └── migrations/
│       ├── 000001_create_bookings.up.sql         # Создание таблицы bookings + индексов
│       └── 000001_create_bookings.down.sql       # Удаление таблицы bookings
├── flight-service/
│   ├── cmd/
│   │   └── main.go                                # Точка входа Flight Service: env, БД, миграции, Redis, gRPC сервер
│   ├── internal/
│   │   ├── model/
│   │   │   └── model.go                          # Модели Flight и SeatReservation
│   │   ├── repository/
│   │   │   ├── repository.go                     # SQL для поиска/получения рейсов, reserve/release с транзакциями
│   │   │   └── errors.go                         # Бизнес-ошибки репозитория
│   │   └── server/
│   │       ├── server.go                         # Реализация gRPC методов + cache-aside + инвалидация кеша
│   │       └── auth_interceptor.go               # Проверка x-api-key для всех gRPC методов (UNAUTHENTICATED)
│   └── migrations/
│       ├── 000001_create_flights.up.sql          # Таблица flights + ограничения целостности
│       ├── 000001_create_flights.down.sql        # Удаление flights
│       ├── 000002_create_seat_reservations.up.sql# Таблица seat_reservations + FK/UK/индексы
│       └── 000002_create_seat_reservations.down.sql
├── proto/
│   └── flight/
│       └── flight.proto                          # gRPC контракт Flight Service (Search/Get/Reserve/Release)
├── docs/
│   └── er-diagram.mmd                            # ER-диаграмма в Mermaid (3NF, ключи, связи)
├── gen/
│   └── flight/
│       ├── flight.pb.go                          # Сгенерированные protobuf типы
│       └── flight_grpc.pb.go                     # Сгенерированные gRPC client/server интерфейсы
├── docker-compose.yml                      
├── Makefile                                     
├── go.mod                                    
└── go.sum                              
```

## Запуск

```bash
docker compose up --build
```

Остановка и очистка томов:

```bash
docker compose down -v
```


## E2E через curl

### 1) Добавить тестовый рейс в Flight DB

```bash
docker compose exec -T flight-db psql -U flight -d flight_db -c '\dt'
docker compose exec -T flight-db psql -U flight -d flight_db <<'SQL'
INSERT INTO flights (
  id, flight_number, airline, origin_airport, destination_airport,
  departure_time, arrival_time, total_seats, available_seats, price, status
)
VALUES (
  '11111111-1111-1111-1111-111111111111',
  'SU1001',
  'Aeroflot',
  'VKO',
  'LED',
  '2026-04-01T10:00:00Z',
  '2026-04-01T11:30:00Z',
  100,
  100,
  5500.00,
  'SCHEDULED'
)
ON CONFLICT (id) DO NOTHING;
SQL
```

### 2) Поиск рейсов

```bash
curl -s "http://localhost:8080/flights?origin=VKO&destination=LED&date=2026-04-01" | jq
```

### 3) Получить рейс по ID

```bash
curl -s "http://localhost:8080/flights/11111111-1111-1111-1111-111111111111" | jq
```

### 4) Создать бронирование

```bash
curl -s -X POST http://localhost:8080/bookings \
  -H "Content-Type: application/json" \
  -d '{
    "user_id":"user-1",
    "flight_id":"11111111-1111-1111-1111-111111111111",
    "passenger_name":"Ivan Petrov",
    "passenger_email":"ivan@example.com",
    "seat_count":2
  }' | jq
```

### 5) Получить бронирование по ID

```bash
curl -s "http://localhost:8080/bookings/<BOOKING_ID>" | jq
```

### 6) Список бронирований пользователя

```bash
curl -s "http://localhost:8080/bookings?user_id=user-1" | jq
```

### 7) Отменить бронирование

```bash
curl -s -X POST "http://localhost:8080/bookings/<BOOKING_ID>/cancel" | jq
```

### 8) Проверить, что места вернулись

```bash
curl -s "http://localhost:8080/flights/11111111-1111-1111-1111-111111111111" | jq '.available_seats'
```

### 9) Flight status через SQL

```bash
docker compose exec -T flight-db psql -U flight -d flight_db -c "UPDATE flights SET status = 'DEPARTED' WHERE id = '11111111-1111-1111-1111-111111111111';"
docker compose exec -T flight-db psql -U flight -d flight_db -c "UPDATE flights SET status = 'CANCELLED' WHERE id = '11111111-1111-1111-1111-111111111111';"
docker compose exec -T flight-db psql -U flight -d flight_db -c "UPDATE flights SET status = 'COMPLETED' WHERE id = '11111111-1111-1111-1111-111111111111';"
docker compose exec -T flight-db psql -U flight -d flight_db -c "SELECT id, flight_number, status FROM flights WHERE id = '11111111-1111-1111-1111-111111111111';"
```


## Проверка кеша Redis (hit/miss)

```bash
docker compose logs -f flight-service
```

В другом терминале:

```bash
curl -s "http://localhost:8080/flights/11111111-1111-1111-1111-111111111111" > /dev/null
curl -s "http://localhost:8080/flights/11111111-1111-1111-1111-111111111111" > /dev/null
```

## Быстрые проверки ошибок

Невалидная дата:

```bash
curl -s "http://localhost:8080/flights?origin=VKO&destination=LED&date=2026/04/01" | jq
```

Недостаточно мест:

```bash
curl -s -X POST http://localhost:8080/bookings \
  -H "Content-Type: application/json" \
  -d '{
    "user_id":"user-2",
    "flight_id":"11111111-1111-1111-1111-111111111111",
    "passenger_name":"Test User",
    "passenger_email":"test@example.com",
    "seat_count":9999
  }' | jq
```

Проверка невалидного статуса:

```bash
docker compose exec -T flight-db psql -U flight -d flight_db -c "UPDATE flights SET status = 'BOARDING' WHERE id = '11111111-1111-1111-1111-111111111111';"
```


