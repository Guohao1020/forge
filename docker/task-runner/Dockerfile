FROM node:20-slim AS base

# Install Python, Go, and common test tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 python3-pip python3-venv \
    curl ca-certificates git jq \
    && rm -rf /var/lib/apt/lists/*

# Install common Node.js test frameworks and React testing deps globally
RUN npm install -g jest vitest tsx typescript @types/node ts-jest \
    @testing-library/react @testing-library/jest-dom jest-environment-jsdom

# Install Python test tools
RUN python3 -m venv /opt/venv && \
    /opt/venv/bin/pip install pytest pytest-json-report pytest-cov

ENV PATH="/opt/venv/bin:$PATH"

# Working directory
WORKDIR /workspace

# Copy entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
