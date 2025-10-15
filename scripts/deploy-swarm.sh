#!/bin/bash

# Configuración
NETWORK_NAME="agenda-network"
MANAGER_IP="10.171.210.16"  # Cambiar por la IP del manager

# Crear red si no existe
docker network create --driver=overlay --attachable $NETWORK_NAME 2>/dev/null || true

# Crear volúmenes para datos persistentes
echo "Creando volúmenes para datos persistentes..."
docker volume create agenda_redis_data 2>/dev/null || true
docker volume create agenda_api_logs 2>/dev/null || true
docker volume create agenda_users_data 2>/dev/null || true
docker volume create agenda_users_logs 2>/dev/null || true
docker volume create agenda_events_data 2>/dev/null || true
docker volume create agenda_groups_data 2>/dev/null || true
docker volume create agenda_notifications_data 2>/dev/null || true

# Desplegar servicios
echo "Desplegando servicios en el swarm..."

# 1. Redis (Bus de Mensajes) - Solo en el manager
echo "Iniciando Redis..."
docker service create \
  --name agenda-bus-redis \
  --network $NETWORK_NAME \
  --constraint 'node.role == manager' \
  --mount source=agenda_redis_data,target=/data \
  --limit-memory 128M \
  redis:7-alpine

# 2. API Gateway - Solo en el manager
echo "Iniciando API Gateway..."
docker service create \
  --name agenda-api-gateway \
  --network $NETWORK_NAME \
  -p 8000:8000 \
  --constraint 'node.role == manager' \
  --mount source=agenda_api_logs,target=/app/logs \
  --limit-memory 256M \
  agenda-api_gateway:latest

# 3. Users Service - Solo en el manager
echo "Iniciando Users Service..."
docker service create \
  --name agenda-users-service \
  --network $NETWORK_NAME \
  -p 8001:8001 \
  --constraint 'node.role == manager' \
  --env REDIS_HOST=agenda-bus-redis \
  --env REDIS_PORT=6379 \
  --env REDIS_DB=0 \
  --env LOG_DIR=/app/logs \
  --mount source=agenda_users_data,target=/app/data \
  --mount source=agenda_users_logs,target=/app/logs \
  --limit-memory 512M \
  agenda-users:latest

# 4. Events Service - Solo en el worker
echo "Iniciando Events Service..."
docker service create \
  --name agenda-events-service \
  --network $NETWORK_NAME \
  -p 8002:8002 \
  --constraint 'node.role == worker' \
  --mount source=agenda_events_data,target=/app/data \
  --limit-memory 512M \
  --env REDIS_URL=redis://agenda-bus-redis:6379/0 \
  agenda-events:latest

# 5. Groups Service - Solo en el worker
echo "Iniciando Groups Service..."
docker service create \
  --name agenda-groups-service \
  --network $NETWORK_NAME \
  -p 8003:8003 \
  --constraint 'node.role == worker' \
  --env ENVIRONMENT=production \
  --env DATABASE_PATH=/app/data/groups.db \
  --env REDIS_URL=redis://agenda-bus-redis:6379/0 \
  --mount source=agenda_groups_data,target=/app/data \
  --limit-memory 512M \
  agenda-group:latest

# 6. Notifications Service - Solo en el manager
# echo "Iniciando Notifications Service..."
# docker service create \
#   --name agenda-notifications-service \
#   --network $NETWORK_NAME \
#   -p 8004:8004 \
#   --constraint 'node.role == manager' \
#   --env REDIS_HOST=agenda-bus-redis \
#   --mount source=agenda_notifications_data,target=/app/data \
#   --limit-memory 256M \
#   agenda-notifications:latest

# 7. Streamlit App - Solo en el manager
echo "Iniciando Streamlit App..."
docker service create \
  --name agenda-streamlit-app \
  --network $NETWORK_NAME \
  -p 8501:8501 \
  --constraint 'node.role == manager' \
  --env REDIS_HOST=agenda-bus-redis \
  --limit-memory 256M \
  agenda-streamlit_app:latest

echo "Verificando estado de los servicios..."
docker service ls

echo ""
echo "Resumen de despliegue:"
echo "- API Gateway: http://localhost:8000"
echo "- Users API: http://localhost:8001"
echo "- Events API: http://localhost:8002"
echo "- Groups API: http://localhost:8003"
echo "- Notifications API: http://localhost:8004"
echo "- Streamlit UI: http://localhost:8501"
echo ""
echo "Para ver los logs de un servicio: docker service logs -f <nombre_servicio>"
echo "Para escalar un servicio: docker service scale <nombre_servicio>=<réplicas>"