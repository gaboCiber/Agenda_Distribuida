#!/bin/bash

# Nombre de la red de Docker
NETWORK_NAME="agenda-net"

# Crear la red si no existe
echo "Creating Docker network: $NETWORK_NAME..."
docker network inspect $NETWORK_NAME >/dev/null 2>&1 || \
    docker network create --driver bridge $NETWORK_NAME

# --- Iniciar servicios desde imágenes existentes ---

# 1. Bus de Mensajes (Redis)
echo "Starting Redis container..."
docker run -d --name agenda-bus-redis --network $NETWORK_NAME redis:7-alpine

# 2. API Gateway
echo "Starting API Gateway..."
docker run -d --name agenda-api-gateway --network $NETWORK_NAME -p 8000:8000 agenda-api_gateway:latest

# 3. Users Service
echo "Starting Users Service..."
docker run -d --name agenda-users-service --network $NETWORK_NAME -p 8001:8001 agenda-users:latest

# 4. Events Service
echo "Starting Events Service..."
docker run -d --name agenda-events-service --network $NETWORK_NAME -p 8002:8002 agenda-events:latest

# 5. Groups Service
echo "Starting Groups Service..."
docker run -d --name agenda-groups-service --network $NETWORK_NAME -p 8003:8003 agenda-groups:latest

# 6. Notifications Service
echo "Starting Notifications Service..."
docker run -d --name agenda-notifications-service --network $NETWORK_NAME -p 8004:8004 agenda-notifications:latest

echo "\nAll services are starting. Use 'docker ps' to check their status."
