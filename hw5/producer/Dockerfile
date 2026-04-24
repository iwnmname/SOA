FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/producer ./cmd/producer

FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /bin/producer /bin/producer

EXPOSE 8080

ENTRYPOINT ["/bin/producer"]
