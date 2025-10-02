#!/bin/bash

# Verificar que se proporcione un nombre de servicio
if [ -z "$1" ]; then
    echo "Uso: $0 <nombre-servicio>"
    echo "Servicios disponibles: api-gateway, users, events, groups, notifications, redis"
    exit 1
fi

# Configuración común
NETWORK_NAME="agenda-net"
CURRENT_DIR="$(pwd)"

# Crear la red si no existe
echo "Verificando red Docker: $NETWORK_NAME..."
docker network inspect $NETWORK_NAME >/dev/null 2>&1 || \
    docker network create --driver bridge $NETWORK_NAME

# Iniciar el servicio solicitado
case "$1" in
    "redis")
        echo "Iniciando Redis..."
        docker run -d --name agenda-bus-redis --network $NETWORK_NAME redis:7-alpine
        ;;
    "api-gateway")
        echo "Iniciando API Gateway..."
        docker run -d --name agenda-api-gateway --network $NETWORK_NAME -p 8000:8000 \
          -v "$CURRENT_DIR/services/api_gateway:/app" agenda-api_gateway:latest
        ;;
    "users")
        echo "Iniciando Users Service..."
        docker run -d --name agenda-users-service --network $NETWORK_NAME -p 8001:8001 \
          -v "$CURRENT_DIR/services/users_service/:/app" \
          -w /app \
          agenda-users:latest 
        ;;
    "events")
        echo "Iniciando Events Service..."
        docker run -d --name agenda-events-service --network $NETWORK_NAME -p 8002:8002 \
          -v "$CURRENT_DIR/services/events_service:/app" agenda-events:latest
        ;;
    "groups")
        echo "Iniciando Groups Service..."
        docker run -d --name agenda-groups-service --network $NETWORK_NAME -p 8003:8003 \
          -v "$CURRENT_DIR/services/groups_service:/app" agenda-groups:latest
        ;;
    "notifications")
        echo "Iniciando Notifications Service..."
        docker run -d --name agenda-notifications-service --network $NETWORK_NAME -p 8004:8004 \
          -v "$CURRENT_DIR/services/notifications_service:/app" agenda-notifications:latest
        ;;
    *)
        echo "Error: Servicio no reconocido: $1"
        echo "Servicios disponibles: api-gateway, users, events, groups, notifications, redis"
        exit 1
        ;;
esac

echo "$1 iniciado. Usa 'docker ps' para verificar el estado."
