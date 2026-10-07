FROM python:3.12-slim

LABEL description="TEFAS + KAP fund holdings scraper (Playwright + pypdf)"

# System deps: chromium for Playwright + PDF support
RUN apt-get update && apt-get install -y --no-install-recommends \
        libpq-dev \
        ca-certificates \
        wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Python deps
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Install Playwright browsers (chromium only)
RUN playwright install --with-deps chromium

COPY fund_scraper.py ai_pdf_parser.py bist_universe.py ./

ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

CMD ["python", "fund_scraper.py"]
