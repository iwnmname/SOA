FROM golang:1.25-alpine AS builder

RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p pkg/generated
RUN oapi-codegen -package generated -generate types -o pkg/generated/types.gen.go api/openapi.yaml
RUN oapi-codegen -package generated -generate chi-server -o pkg/generated/server.gen.go api/openapi.yaml
RUN CGO_ENABLED=0 go build -o /bin/app ./cmd/app

FROM alpine:3.20
COPY --from=builder /bin/app /bin/app
COPY --from=builder /app/migrations /migrations
CMD ["/bin/app"]