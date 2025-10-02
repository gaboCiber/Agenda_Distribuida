#!/bin/bash

# Nombre de la red de Docker
NETWORK_NAME="agenda-net"

# Crear la red si no existe
echo "Creating Docker network: $NETWORK_NAME..."
docker network inspect $NETWORK_NAME >/dev/null 2>&1 || \
    docker network create --driver bridge $NETWORK_NAME

# Obtener ruta absoluta del directorio actual
CURRENT_DIR="$(pwd)"

# --- Iniciar servicios desde imágenes existentes con volúmenes dinámicos ---

# 1. Bus de Mensajes (Redis)
echo "Starting Redis container..."
docker run -d --name agenda-bus-redis --network $NETWORK_NAME redis:7-alpine

# 2. API Gateway
echo "Starting API Gateway..."
docker run -d --name agenda-api-gateway --network $NETWORK_NAME -p 8000:8000 \
  -v "$CURRENT_DIR/services/api_gateway:/app" agenda-api_gateway:latest

# 3. Users Service
echo "Starting Users Service..."
docker run -d --name agenda-users-service --network $NETWORK_NAME -p 8001:8001 \
  -e REDIS_HOST=agenda-bus-redis \
  -e REDIS_PORT=6379 \
  -e REDIS_DB=0 \
  -e LOG_DIR=/app/logs \
  -v "$CURRENT_DIR/services/users_service:/app" \
  -v "$CURRENT_DIR/services/users_service/logs:/app/logs" \
  agenda-users:latest

# 4. Events Service
echo "Starting Events Service..."
docker run -d --name agenda-events-service --network $NETWORK_NAME -p 8002:8002 \
  -v "$CURRENT_DIR/services/events_service:/app" agenda-events:latest

# 5. Groups Service
echo "Starting Groups Service..."
docker run -d --name agenda-groups-service --network $NETWORK_NAME -p 8003:8003 \
  -v "$CURRENT_DIR/services/groups_service:/app" agenda-groups:latest

# 6. Notifications Service
echo "Starting Notifications Service..."
docker run -d --name agenda-notifications-service --network $NETWORK_NAME -p 8004:8004 \
  -v "$CURRENT_DIR/services/notifications_service:/app" agenda-notifications:latest

echo "\nAll services are starting. Use 'docker ps' to check their status."
