# Users Service
FROM python:3.12-slim

# Set environment variables
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH=/app \
    PORT=8001

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install Python dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Asegurarse de que el directorio de la aplicación existe
RUN mkdir -p /app/services

# Create necessary directories with correct permissions
ENV DATABASE_PATH=/app/data/db/agenda_users.db
ENV LOG_DIR=/app/logs

# Create required directories with correct permissions
RUN mkdir -p /app/data/db \
    && mkdir -p $LOG_DIR \
    && touch $LOG_DIR/users_service.log \
    && chown -R 1000:1000 /app \
    && chmod -R 755 $LOG_DIR \
    && chmod 666 $LOG_DIR/users_service.log

# Copy application code
COPY . .

# Set permissions for the app directory
RUN chown -R 1000:1000 /app

# Run as non-root user for security
# USER 1000

# Install uvicorn if not already installed
RUN pip install --no-cache-dir uvicorn

# Run the application with uvicorn
CMD ["uvicorn", "main_fixed:app", "--host", "0.0.0.0", "--port", "8001", "--log-level", "info"]
