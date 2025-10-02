#!/bin/bash

# Verificar que se proporcione un nombre de servicio
if [ -z "$1" ]; then
    echo "Uso: $0 <nombre-servicio>"
    echo "Servicios disponibles: api-gateway, users, events, groups, notifications, redis"
    exit 1
fi

# Mapeo de nombres de servicio a nombres de contenedor
case "$1" in
    "redis") CONTAINER="agenda-bus-redis" ;;
    "api-gateway") CONTAINER="agenda-api-gateway" ;;
    "users") CONTAINER="agenda-users-service" ;;
    "events") CONTAINER="agenda-events-service" ;;
    "groups") CONTAINER="agenda-groups-service" ;;
    "notifications") CONTAINER="agenda-notifications-service" ;;
    *)
        echo "Error: Servicio no reconocido: $1"
        echo "Servicios disponibles: api-gateway, users, events, groups, notifications, redis"
        exit 1
        ;;
esac

# Detener y eliminar el contenedor si existe
if docker ps -a --format '{{.Names}}' | grep -q "^$CONTAINER$"; then
    echo "Deteniendo $1..."
    docker stop $CONTAINER >/dev/null
    docker rm $CONTAINER >/dev/null
    echo "$1 detenido y eliminado."
else
    echo "El servicio $1 no está en ejecución."
fi
