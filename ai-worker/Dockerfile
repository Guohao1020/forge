# forge AI Worker — Python Temporal activity executor
# Connects to Temporal and processes AI activities (analyze, plan, generate, review, test, profile scan)
#
# Build: docker build -t forge-ai-worker:latest .
# Run:   docker run --env-file .env forge-ai-worker:latest

FROM python:3.12-slim

# System deps for httpx and async
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    bubblewrap \
    ripgrep \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install Python deps
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy source
COPY src/ src/

# Entry point script: runs uvicorn (FastAPI on :8090) + Temporal worker side-by-side
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

# Non-root user
RUN useradd -m forge
USER forge

EXPOSE 8090

CMD ["/app/start.sh"]
