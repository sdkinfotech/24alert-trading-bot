FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE_NAME=gateway
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/service ./cmd/${SERVICE_NAME}

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl netcat-openbsd wget && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL -o /usr/local/share/ca-certificates/russian-trusted-root.crt \
    https://gu-st.ru/content/lending/russian_trusted_root_ca_pem.crt && \
    curl -fsSL -o /usr/local/share/ca-certificates/russian-trusted-sub.crt \
    https://gu-st.ru/content/lending/russian_trusted_sub_ca_pem.crt && \
    update-ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/service .
COPY config/ ./config/
EXPOSE 8080 9001 9002 9003 9004 9101 9102 9103 9104
ENTRYPOINT ["./service"]
CMD ["--config", "config/config.yaml"]
