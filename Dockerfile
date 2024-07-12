FROM golang:1.22.5 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o goemini /app/cmd/goemini

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/goemini /app/goemini
COPY --from=builder /app/start_server.sh /app/start_server.sh

ENTRYPOINT ["/app/start_server.sh"]