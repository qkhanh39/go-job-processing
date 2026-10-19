FROM golang:1.26 AS builder

WORKDIR /app

ARG ENVIRONMENT

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o app ./cmd/http-server/main.go

FROM alpine:3.19 AS run-time

WORKDIR /app

COPY --from=builder /app/app .
EXPOSE 8080

CMD ["./app"]