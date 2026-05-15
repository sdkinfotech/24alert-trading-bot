# AI Scanner for 24alert: cron + Cursor Agent CLI + Python3 tools.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    bash \
    cron \
    coreutils \
    python3 \
    python3-pip \
  && rm -rf /var/lib/apt/lists/*

ENV HOME=/root
ENV PATH="/root/.local/bin:${PATH}"

RUN curl -fsSL https://cursor.com/install | bash \
  && test -x /root/.local/bin/agent

WORKDIR /
