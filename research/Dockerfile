FROM python:3.11-slim

# IC gate harness for metric IC computation and promotion validation
# Requires PostgreSQL and optional Redis

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends libpq-dev \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies FIRST, before copying code
# This ensures stdlib modules are properly cached before local modules are visible
COPY pyproject.toml .
RUN pip install --no-cache-dir \
    pandas>=2.2.0 \
    numpy>=1.26.0 \
    scipy>=1.13.0 \
    statsmodels>=0.14.0 \
    psycopg[binary]>=3.1.0 \
    redis>=5.0.0 \
    python-dateutil>=2.8.0 \
    pyyaml>=6.0

# NOW copy application code
COPY ic_gate.py .
COPY forward_returns.py .
COPY ic_gate/ ./ic_gate/
COPY returns/ ./returns/
COPY utils/ ./utils/
COPY bist_calendar/ ./bist_calendar/

# Create config directory for volume mount (config/neyialiyorlar.yaml is mounted at runtime)
RUN mkdir -p /app/config && mkdir -p /src && \
    cp ic_gate.py forward_returns.py /src/ && \
    cp -r ic_gate returns utils bist_calendar /src/

ENV PYTHONUNBUFFERED=1
ENV TIMEZONE=Europe/Istanbul

CMD ["python", "/src/ic_gate.py", "--metric=all"]
