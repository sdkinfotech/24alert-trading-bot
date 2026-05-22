FROM node:22-slim AS dashboard
WORKDIR /app/web/strategy-dashboard
COPY web/strategy-dashboard/package.json web/strategy-dashboard/package-lock.json ./
RUN npm ci --ignore-scripts
COPY web/strategy-dashboard/ .
RUN npm run build

FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=dashboard /app/web/strategy-dashboard/dist /app/internal/strategy/dashboard_dist
ARG SERVICE_NAME=gateway
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/service ./cmd/${SERVICE_NAME}

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl netcat-openbsd wget python3-minimal && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL -o /usr/local/share/ca-certificates/russian-trusted-root.crt \
    https://gu-st.ru/content/lending/russian_trusted_root_ca_pem.crt && \
    curl -fsSL -o /usr/local/share/ca-certificates/russian-trusted-sub.crt \
    https://gu-st.ru/content/lending/russian_trusted_sub_ca_pem.crt && \
    update-ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/service .
COPY config/ ./config/
COPY scripts/backtestlib ./scripts/backtestlib
COPY scripts/backtest ./scripts/backtest
COPY scripts/ai-scanner/backtest.py ./scripts/ai-scanner/backtest.py
EXPOSE 8080 9001 9002 9003 9004 9020 9120 9101 9102 9103 9104
ENTRYPOINT ["./service"]
CMD ["--config", "config/config.yaml"]
