#!/bin/bash

# Nombres de los contenedores
CONTAINERS=(
    "agenda-api-gateway"
    "agenda-users-service"
    "agenda-events-service"
    "agenda-groups-service"
    "agenda-notifications-service"
    "agenda-bus-redis"
)

# Nombre de la red
NETWORK_NAME="agenda-net"

echo "Stopping and removing containers..."
for container in "${CONTAINERS[@]}"; do
    docker stop $container >/dev/null 2>&1
    docker rm $container >/dev/null 2>&1
done

echo "Removing Docker network: $NETWORK_NAME..."
docker network rm $NETWORK_NAME >/dev/null 2>&1

echo "Cleanup complete."
