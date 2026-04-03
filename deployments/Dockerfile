FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE_NAME=gateway
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/service ./cmd/${SERVICE_NAME}

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/service .
COPY config/ ./config/
EXPOSE 8080 9001 9002 9003 9004
ENTRYPOINT ["./service"]
CMD ["--config", "config/config.yaml"]
