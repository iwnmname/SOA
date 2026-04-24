FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/aggregator ./cmd/aggregator

FROM gcr.io/distroless/static-debian12
COPY --from=builder /bin/aggregator /aggregator
EXPOSE 8090
ENTRYPOINT ["/aggregator"]

