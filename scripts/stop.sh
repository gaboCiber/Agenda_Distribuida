#!/bin/bash

# Check if service name is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 [all|redis|db|user|group|api]"
    exit 1
fi

SERVICE=$1

stop_container() {
    local container_name=$1
    echo "Stopping $container_name..."
    docker stop $container_name 2>/dev/null
    echo "Removing $container_name..."
    docker rm $container_name 2>/dev/null
    echo "$container_name stopped and removed"
}

case $SERVICE in
    all)
        stop_container "agenda-api-gateway-service"
        stop_container "agenda-group-service"
        stop_container "agenda-user-service"
        stop_container "agenda-db-service"
        stop_container "agenda-redis-service"
        stop_container "agenda-redis-supervisor-service"
        ;;
        
    redis)
        stop_container "agenda-redis-service"
        ;;
        
    db)
        stop_container "agenda-db-service"
        ;;
        
    user)
        stop_container "agenda-user-service"
        ;;
        
    group)
        stop_container "agenda-group-service"
        ;;

    api)
        stop_container "agenda-api-gateway-service"
        ;;
    redis-supervisor)
        stop_container "agenda-redis-supervisor-service"
        ;;
    *)
        echo "Error: Unknown service '$SERVICE'"
        echo "Available services: all, redis, db, user, group, api, redis-supervisor"
        exit 1
        ;;
esac

echo "Done!"
