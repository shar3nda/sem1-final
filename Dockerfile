FROM golang:1.23-alpine3.22 AS builder

WORKDIR /app

RUN apk add --no-cache git

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o service ./cmd/app/main.go

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/service /app/service

EXPOSE 8080

CMD ["./service"]
