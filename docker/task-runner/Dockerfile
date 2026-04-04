# forge-task-runner — Multi-language test execution environment
# Used by S12 (automated testing) and S13 (artifact building)
#
# Contains Go, Node.js, and Python runtimes for running AI-generated tests
# in isolated containers (K8s Jobs or Docker containers).

FROM ubuntu:22.04

# Avoid interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# --- System packages ---
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    jq \
    make \
    unzip \
    wget \
    && rm -rf /var/lib/apt/lists/*

# --- Go 1.22 ---
ENV GOLANG_VERSION=1.22.5
RUN curl -fsSL "https://go.dev/dl/go${GOLANG_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/go"
ENV PATH="${GOPATH}/bin:${PATH}"

# Go test tools
RUN go install github.com/jstemmer/go-junit-report/v2@latest \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1

# --- Node.js 20 ---
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && npm install -g pnpm \
    && rm -rf /var/lib/apt/lists/*

# --- Python 3.12 ---
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3.12 \
    python3.12-venv \
    python3-pip \
    && rm -rf /var/lib/apt/lists/* \
    && ln -sf /usr/bin/python3.12 /usr/bin/python \
    && ln -sf /usr/bin/python3.12 /usr/bin/python3

# Python test tools
RUN pip install --no-cache-dir \
    pytest \
    pytest-cov \
    pytest-asyncio \
    playwright \
    httpx \
    && python -m playwright install --with-deps chromium

# --- Working directory ---
WORKDIR /workspace

# --- Entrypoint: clone repo → install deps → run tests → report ---
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
