#!/bin/bash

# Docker network name
NETWORK_NAME="agenda-network"

# Get absolute path of the current directory
CURRENT_DIR="$(pwd)"

# Check if service name is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 [all|redis|db|user|group|api]"
    exit 1
fi

SERVICE=$1

# Create the network if it doesn't exist
echo "Checking Docker network..."
docker network inspect $NETWORK_NAME >/dev/null 2>&1 || \
    docker network create --driver bridge $NETWORK_NAME

start_redis() {
    echo "Starting Redis container..."
    docker run -d --name agenda-redis-service --network $NETWORK_NAME \
      -p 6380:6379 \
      redis:7-alpine
    echo "Redis started at localhost:6380"
}

start_db() {
    echo "Starting DB Service..."
    docker run -d --name agenda-db-service --network $NETWORK_NAME \
      -p 8000:8000 \
      -v "$CURRENT_DIR/services/db_service/data:/data" \
      agenda-db
    echo "DB Service started at localhost:8000"
}

start_user() {
    echo "Starting User Service..."
    docker run -d --name agenda-user-service --network $NETWORK_NAME \
      -p 8001:8001 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e DB_SERVICE_URL=http://agenda-db-service:8000 \
      -e LOG_LEVEL=debug \
      agenda-user_event
    echo "User Service started at localhost:8001"
}

start_group() {
    echo "Starting Group Service..."
    docker run -d --name agenda-group-service --network $NETWORK_NAME \
      -p 8002:8002 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e DB_SERVICE_URL=http://agenda-db-service:8000 \
      -e LOG_LEVEL=debug \
      agenda-group_event
    echo "Group Service started at localhost:8002"
}

start_api() {
    echo "Starting API Gateway Service..."
    docker run -d --name agenda-api-gateway-service --network $NETWORK_NAME \
      -p 8080:8080 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e LOG_LEVEL=debug \
      agenda-api-gateway
    echo "API Gateway Service started at localhost:8080"
}

case $SERVICE in
    all)
        echo "Starting all services in order: redis → db → user → group → api"
        start_redis
        sleep 2
        start_db
        sleep 2
        start_user
        sleep 2
        start_group
        sleep 2
        start_api
        echo "All services started successfully!"
        echo "- Redis: localhost:6380"
        echo "- DB Service: localhost:8000"
        echo "- User Service: localhost:8001"
        echo "- Group Service: localhost:8002"
        echo "- API Gateway: localhost:8080"
        ;;
        
    redis)
        start_redis
        ;;
        
    db)
        start_db
        ;;
        
    user)
        start_user
        ;;
        
    group)
        start_group
        ;;

    api)
        start_api
        ;;

    *)
        echo "Error: Unknown service '$SERVICE'"
        echo "Available services: redis, db, user, group, api"
        exit 1
        ;;
esac

echo "Done!"
