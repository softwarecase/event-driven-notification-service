FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/worker ./cmd/worker

# API image
FROM alpine:3.21 AS api
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/api /usr/local/bin/api
EXPOSE 8080
CMD ["api"]

# Worker image
FROM alpine:3.21 AS worker
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/worker /usr/local/bin/worker
CMD ["worker"]
