## Структура проекта

```
hw2/
├── api/
│   └── openapi.yaml                 # OpenAPI-спецификация
├── cmd/
│   └── app/
│       └── main.go                  # Точка входа: миграции, DI, запуск сервера
├── internal/
│   ├── config/
│   │   └── config.go                # Конфигурация из env-переменных
│   ├── domain/
│   │   ├── product.go               # Доменная модель Product
│   │   ├── order.go                 # Доменная модель Order + OrderItem
│   │   ├── promo.go                 # Доменная модель PromoCode
│   │   └── user.go                  # Доменная модель User + RefreshToken
│   ├── handler/
│   │   ├── auth.go                  # Обработчики /auth/* (register, login, refresh)
│   │   ├── product.go               # Обработчики /products/* + RBAC
│   │   ├── order.go                 # Обработчики /orders/* + RBAC
│   │   ├── promo.go                 # Обработчик POST /promo-codes + RBAC
│   │   ├── validation.go            # Валидация входных данных
│   │   └── helpers.go               # writeJSON, writeError, MustGetAuth
│   ├── middleware/
│   │   ├── auth.go                  # JWT-валидация, извлечение user_id/role в контекст
│   │   └── logging.go               # JSON-логирование запросов, X-Request-Id
│   ├── repository/
│   │   ├── db.go                    # Интерфейс DBTX (pool или tx)
│   │   ├── product.go               # SQL-запросы для products
│   │   ├── order.go                 # SQL-запросы для orders + order_items
│   │   ├── promo.go                 # SQL-запросы для promo_codes
│   │   ├── user.go                  # SQL-запросы для users + refresh_tokens
│   │   └── user_op.go               # SQL-запросы для user_operations (rate limit)
│   └── service/
│       ├── product.go               # Бизнес-логика products (мягкое удаление)
│       ├── order.go                 # Бизнес-логика заказов (все проверки, транзакции)
│       └── auth.go                  # Регистрация, логин, JWT, refresh
├── migrations/
│   ├── 000001_create_products.up.sql
│   ├── 000001_create_products.down.sql
│   ├── 000002_create_orders_tables.up.sql
│   ├── 000002_create_orders_tables.down.sql
│   ├── 000003_create_users.up.sql
│   └── 000003_create_users.down.sql
├── pkg/
│   └── generated/                   # Сгенерированный код (в .gitignore)
│       ├── types.gen.go             # DTO из OpenAPI-схем
│       └── server.gen.go            # ServerInterface + роутинг chi
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── go.mod
```

## Запуск

### Docker

```bash
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

make generate

go mod tidy

docker-compose up --build
```

### Остановка

```bash
docker-compose down -v
```

---

## E2E демонстрация

### 1. Регистрация пользователей

```bash
curl -s -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"buyer@test.com","password":"pass123","role":"USER"}' | jq

curl -s -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"seller@test.com","password":"pass123","role":"SELLER"}' | jq

curl -s -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"pass123","role":"ADMIN"}' | jq
```

### 2. Логин и получение токенов

```bash
curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"seller@test.com","password":"pass123"}' | jq

export SELLER_TOKEN="<access_token>"

curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"buyer@test.com","password":"pass123"}' | jq

export USER_TOKEN="<access_token>"

curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"pass123"}' | jq

export ADMIN_TOKEN="<access_token>"
```

### 3. SELLER создаёт товары

```bash
curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d '{"name":"iPhone 15","description":"Smartphone","price":999.99,"stock":50,"category":"Electronics"}' | jq

curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d '{"name":"AirPods Pro","price":249.99,"stock":100,"category":"Electronics"}' | jq

curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d '{"name":"Limited Edition Watch","price":5000,"stock":2,"category":"Accessories"}' | jq
```

### 4. Просмотр товаров

```bash
curl -s -H "Authorization: Bearer $USER_TOKEN" "http://localhost:8080/products" | jq

curl -s -H "Authorization: Bearer $USER_TOKEN" "http://localhost:8080/products?page=0&size=2" | jq

curl -s -H "Authorization: Bearer $USER_TOKEN" "http://localhost:8080/products?category=Electronics" | jq

curl -s -H "Authorization: Bearer $USER_TOKEN" "http://localhost:8080/products?status=ACTIVE" | jq

curl -s -H "Authorization: Bearer $USER_TOKEN" "http://localhost:8080/products/<PRODUCT_ID>" | jq
```

### 5. Обновление товара

```bash
curl -s -X PUT "http://localhost:8080/products/<PRODUCT_ID>" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d '{"name":"iPhone 15 Pro","description":"Upgraded","price":1199.99,"stock":45,"category":"Electronics","status":"ACTIVE"}' | jq
```

### 6. Мягкое удаление

```bash
curl -s -X DELETE "http://localhost:8080/products/<PRODUCT_ID>" \
  -H "Authorization: Bearer $SELLER_TOKEN" | jq
```

### 7. Создание промокода

```bash
curl -s -X POST http://localhost:8080/promo-codes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d '{"code":"SAVE10","discount_type":"PERCENTAGE","discount_value":10,"min_order_amount":100,"max_uses":5,"valid_from":"2024-01-01T00:00:00Z","valid_until":"2026-01-01T00:00:00Z"}' | jq
```

### 8. Создание заказа

```bash
curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{"items":[{"product_id":"<PRODUCT_1>","quantity":2},{"product_id":"<PRODUCT_2>","quantity":1}],"promo_code":"SAVE10"}' | jq
```

### 9. Проверка stock после заказа

```bash
curl -s -H "Authorization: Bearer $USER_TOKEN" "http://localhost:8080/products/<PRODUCT_1>" | jq '.stock'
curl -s -H "Authorization: Bearer $USER_TOKEN" "http://localhost:8080/products/<PRODUCT_2>" | jq '.stock'
```

### 10. Просмотр заказа

```bash
curl -s -H "Authorization: Bearer $USER_TOKEN" "http://localhost:8080/orders/<ORDER_ID>" | jq
```

### 11. Обновление заказа (подождать 1 минуту)

```bash
curl -s -X PUT "http://localhost:8080/orders/<ORDER_ID>" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{"items":[{"product_id":"<PRODUCT_1>","quantity":1}]}' | jq
```

### 12. Отмена заказа

```bash
curl -s -X POST "http://localhost:8080/orders/<ORDER_ID>/cancel" \
  -H "Authorization: Bearer $USER_TOKEN" | jq
```

### 13. Refresh token

```bash
curl -s -X POST http://localhost:8080/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<REFRESH_TOKEN>"}' | jq
```

---

## Альтернативные сценарии

### Без токена → 401 TOKEN_INVALID

```bash
curl -s http://localhost:8080/products | jq
```

### USER создаёт товар → 403 ACCESS_DENIED

```bash
curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{"name":"Test","price":10,"stock":1,"category":"Test"}' | jq
```

### SELLER создаёт заказ → 403 ACCESS_DENIED

```bash
curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d '{"items":[{"product_id":"<ID>","quantity":1}]}' | jq
```

### Несуществующий товар → 404 PRODUCT_NOT_FOUND

```bash
curl -s -H "Authorization: Bearer $USER_TOKEN" \
  http://localhost:8080/products/00000000-0000-0000-0000-000000000000 | jq
```

### Повторное удаление → 409 INVALID_STATE_TRANSITION

```bash
curl -s -X DELETE "http://localhost:8080/products/<ARCHIVED_ID>" \
  -H "Authorization: Bearer $SELLER_TOKEN" | jq
```

### Заказ слишком быстро → 429 ORDER_LIMIT_EXCEEDED

```bash
curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{"items":[{"product_id":"<ID>","quantity":1}]}' | jq

curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{"items":[{"product_id":"<ID>","quantity":1}]}' | jq
```

### Недостаточно товара → 409 INSUFFICIENT_STOCK

```bash
curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{"items":[{"product_id":"<ID>","quantity":9999}]}' | jq
```

### Неактивный товар → 409 PRODUCT_INACTIVE

```bash
curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{"items":[{"product_id":"<ARCHIVED_PRODUCT_ID>","quantity":1}]}' | jq
```

### Невалидный промокод → 422 PROMO_CODE_INVALID

```bash
curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{"items":[{"product_id":"<ID>","quantity":1}],"promo_code":"FAKE"}' | jq
```

### Чужой заказ → 403 ORDER_OWNERSHIP_VIOLATION

```bash
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:8080/orders/<OTHER_USER_ORDER_ID>" | jq
```

### Валидация → 400 VALIDATION_ERROR

```bash
curl -s -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d '{"name":"","price":-1,"stock":-5,"category":""}' | jq
```

---

## Подключение к БД

```bash
docker ps

docker exec -it hw2-db-1 psql -U postgres -d marketplace
```

### Полезные запросы

```sql
-- Список всех таблиц
\dt

-- Все зарегистрированные пользователи
SELECT id, email, role, created_at FROM users;

-- Все товары
SELECT id, name, price, stock, status, seller_id, created_at FROM products;

-- Все заказы
SELECT id, user_id, status, total_amount, discount_amount, promo_code_id, created_at FROM orders;

-- Промокоды
SELECT id, code, discount_type, discount_value, min_order_amount, max_uses, current_uses, active FROM promo_codes;

-- История операций для rate limiting
SELECT id, user_id, operation_type, created_at FROM user_operations ORDER BY created_at DESC;

-- Активные refresh-токены
SELECT id, user_id, token, expires_at FROM refresh_tokens;

-- Версия миграций
SELECT * FROM schema_migrations;

-- Список всех индексов
\di

\q
```