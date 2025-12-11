#!/bin/bash

# Docker network name
NETWORK_NAME="agenda-network"

# Get absolute path of the current directory
CURRENT_DIR="$(pwd)"

# Check if service name is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 [all|redis|raft-db|user]"
    echo "  all     - Start all services in order"
    echo "  redis   - Start Redis only"
    echo "  raft-db - Start Raft DB cluster (3 nodes)"
    echo "  user    - Start User Service only"
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

start_raft_db() {
    echo "Starting Raft DB Cluster (3 nodes)..."
    
    # Node 1 - Leader
    echo "Starting Raft DB Node 1 (potential leader)..."
    docker run -d --name agenda-db-raft-node-1 --network $NETWORK_NAME \
      -p 8001:8001 \
      -v "$CURRENT_DIR/services/db_service/data/node1:/data" \
      -e RAFT_ID=node1 \
      -e RAFT_PEERS="node1=agenda-db-raft-node-1:9001,node2=agenda-db-raft-node-2:9002,node3=agenda-db-raft-node-3:9003" \
      -e RAFT_DATA_DIR=/data/node1/raft \
      -e DB_PATH=/data/node1/app.db \
      -e SERVER_PORT=8001 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e LOG_LEVEL=debug \
      agenda-db_event
    echo "Raft DB Node 1 started at localhost:8001"
    
    # Node 2
    echo "Starting Raft DB Node 2..."
    docker run -d --name agenda-db-raft-node-2 --network $NETWORK_NAME \
      -p 8002:8002 \
      -v "$CURRENT_DIR/services/db_service/data/node2:/data" \
      -e RAFT_ID=node2 \
      -e RAFT_PEERS="node1=agenda-db-raft-node-1:9001,node2=agenda-db-raft-node-2:9002,node3=agenda-db-raft-node-3:9003" \
      -e RAFT_DATA_DIR=/data/node2/raft \
      -e DB_PATH=/data/node2/app.db \
      -e SERVER_PORT=8002 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e LOG_LEVEL=debug \
      agenda-db_event
    echo "Raft DB Node 2 started at localhost:8002"
    
    # Node 3
    echo "Starting Raft DB Node 3..."
    docker run -d --name agenda-db-raft-node-3 --network $NETWORK_NAME \
      -p 8003:8003 \
      -v "$CURRENT_DIR/services/db_service/data/node3:/data" \
      -e RAFT_ID=node3 \
      -e RAFT_PEERS="node1=agenda-db-raft-node-1:9001,node2=agenda-db-raft-node-2:9002,node3=agenda-db-raft-node-3:9003" \
      -e RAFT_DATA_DIR=/data/node3/raft \
      -e DB_PATH=/data/node3/app.db \
      -e SERVER_PORT=8003 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e LOG_LEVEL=debug \
      agenda-db_event
    echo "Raft DB Node 3 started at localhost:8003"
    
    echo "Raft DB Cluster started with 3 nodes"
    echo "Ports: 8001, 8002, 8003"
}

start_user() {
    echo "Starting User Service..."
    docker run -d --name agenda-user-service --network $NETWORK_NAME \
      -p 8004:8004 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e REDIS_CHANNEL=users_events \
      -e DB_SERVICE_URL=http://agenda-db-raft-node-1:8001 \
      -e RAFT_NODES_URLS=http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003 \
      -e LOG_LEVEL=debug \
      agenda-user_event
    echo "User Service started at localhost:8004"
}

stop_services() {
    echo "Stopping all services..."
    docker stop agenda-user-service agenda-db-raft-node-1 agenda-db-raft-node-2 agenda-db-raft-node-3 agenda-redis-service 2>/dev/null
    docker rm agenda-user-service agenda-db-raft-node-1 agenda-db-raft-node-2 agenda-db-raft-node-3 agenda-redis-service 2>/dev/null
    echo "All services stopped"
}

clean_data() {
    echo "Cleaning data directories..."
    rm -rf "$CURRENT_DIR/services/db_service/data/node1"
    rm -rf "$CURRENT_DIR/services/db_service/data/node2"
    rm -rf "$CURRENT_DIR/services/db_service/data/node3"
    echo "Data directories cleaned"
}

show_status() {
    echo "=== Service Status ==="
    echo "Redis:"
    docker ps --filter "name=agenda-redis-service" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""
    echo "Raft DB Cluster:"
    docker ps --filter "name=agenda-db-raft-node" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""
    echo "User Service:"
    docker ps --filter "name=agenda-user-service" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""
    echo "=== Raft Status ==="
    echo "Node 1 (8001):"
    curl -s http://localhost:8001/raft/status | jq . 2>/dev/null || echo "Not responding"
    echo ""
    echo "Node 2 (8002):"
    curl -s http://localhost:8002/raft/status | jq . 2>/dev/null || echo "Not responding"
    echo ""
    echo "Node 3 (8003):"
    curl -s http://localhost:8003/raft/status | jq . 2>/dev/null || echo "Not responding"
}

case $SERVICE in
    all)
        echo "Starting all services in order: redis → raft-db → user"
        clean_data
        start_redis
        sleep 2
        start_raft_db
        sleep 5
        start_user
        sleep 2
        echo ""
        echo "=== All Services Started ==="
        show_status
        ;;
    redis)
        start_redis
        ;;
    raft-db)
        clean_data
        start_raft_db
        ;;
    user)
        start_user
        ;;
    stop)
        stop_services
        ;;
    clean)
        stop_services
        clean_data
        ;;
    status)
        show_status
        ;;
    *)
        echo "Unknown service: $SERVICE"
        echo "Available services: all, redis, raft-db, user, stop, clean, status"
        exit 1
        ;;
esac

echo ""
echo "=== Useful Commands ==="
echo "Check Raft status: curl http://localhost:8001/raft/status"
echo "Check User Service logs: docker logs agenda-user-service"
echo "Test User Service: curl http://localhost:8004/health"
echo "Check Raft logs: docker logs agenda-db-raft-node-1"
