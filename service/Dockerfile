FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go build -o service main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/service .
EXPOSE 8080
CMD ["./service"]