#!/bin/bash

# Docker network name
NETWORK_NAME="agenda-network"

# Get absolute path of the current directory
CURRENT_DIR="$(pwd)"

# Check if service name is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 [all|redis|raft-db|user|group]"
    echo "  all     - Start all services in order"
    echo "  redis   - Start Redis cluster + supervisor only"
    echo "  raft-db - Start Raft DB cluster (3 nodes)"
    echo "  user    - Start User Service only"
    echo "  group   - Start Group Service only"
    exit 1
fi

SERVICE=$1

# Create the network if it doesn't exist
echo "Checking Docker network..."
docker network inspect $NETWORK_NAME >/dev/null 2>&1 || \
    docker network create --driver bridge $NETWORK_NAME

# Redis cluster configuration
REDIS_A_NAME="agenda-redis-a-service"
REDIS_B_NAME="agenda-redis-b-service"
REDIS_C_NAME="agenda-redis-c-service"
REDIS_SUPERVISOR_NAME="agenda-redis-supervisor-service"

start_redis() {
    echo "Starting Redis cluster + supervisor..."
    
    # Stop existing Redis containers
    echo "Stopping existing Redis containers..."
    docker stop $REDIS_SUPERVISOR_NAME $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME > /dev/null 2>&1 || true
    docker rm $REDIS_SUPERVISOR_NAME $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME > /dev/null 2>&1 || true

    # Start Redis A (Master)
    echo "Starting Redis A (Master)..."
    docker run -d --name $REDIS_A_NAME --network $NETWORK_NAME \
      -p 6379:6379 \
      redis:7-alpine
    echo "Redis A (Master) started on port 6379"
    
    sleep 2
    
    # Start Redis B (Replica)
    echo "Starting Redis B (Replica)..."
    docker run -d --name $REDIS_B_NAME --network $NETWORK_NAME \
      -p 6380:6379 \
      redis:7-alpine redis-server --replicaof $REDIS_A_NAME 6379
    echo "Redis B (Replica) started on port 6380"
    
    sleep 2
    
    # Start Redis C (Replica)
    echo "Starting Redis C (Replica)..."
    docker run -d --name $REDIS_C_NAME --network $NETWORK_NAME \
      -p 6381:6379 \
      redis:7-alpine redis-server --replicaof $REDIS_A_NAME 6379
    echo "Redis C (Replica) started on port 6381"
    
    sleep 2
    
    # Start Redis Supervisor
    echo "Starting Redis Supervisor..."
    docker run -d --name $REDIS_SUPERVISOR_NAME --network $NETWORK_NAME \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -e REDIS_ADDRS="${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379" \
      -e DB_SERVICE_URL="http://agenda-db-raft-node-1:8001" \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003" \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor
    echo "Redis Supervisor started"
    
    echo "=== Redis cluster started ==="
    echo "Redis A (Master): ${REDIS_A_NAME}:6379"
    echo "Redis B (Replica): ${REDIS_B_NAME}:6379" 
    echo "Redis C (Replica): ${REDIS_C_NAME}:6379"
    echo "Redis Supervisor: ${REDIS_SUPERVISOR_NAME}"
}

start_raft_db() {
    echo "Starting Raft DB Cluster (3 nodes)..."
    
    # Node 1 - Leader
    echo "Starting Raft DB Node 1 (potential leader)..."
    docker run -d --name agenda-db-raft-node-1 --network $NETWORK_NAME \
      -p 8001:8001 \
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
      -e REDIS_URL=redis://agenda-redis-a-service:6379 \
      -e REDIS_CHANNEL=users_events \
      -e DB_SERVICE_URL=http://agenda-db-raft-node-1:8001 \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003" \
      -e LOG_LEVEL=debug \
      agenda-user_event
    echo "User Service started at localhost:8004"
}

start_group() {
    echo "Starting Group Service..."
    docker run -d --name agenda-group-service --network $NETWORK_NAME \
      -p 8005:8005 \
      -e REDIS_URL=redis://agenda-redis-a-service:6379 \
      -e REDIS_CHANNEL=groups_events \
      -e DB_SERVICE_URL=http://agenda-db-raft-node-1:8001 \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003" \
      -e LOG_LEVEL=debug \
      agenda-group_event
    echo "Group Service started at localhost:8005"
}

stop_services() {
    echo "Stopping all services..."
    docker stop agenda-group-service agenda-user-service agenda-db-raft-node-1 agenda-db-raft-node-2 agenda-db-raft-node-3 $REDIS_SUPERVISOR_NAME $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME 2>/dev/null
    docker rm agenda-group-service agenda-user-service agenda-db-raft-node-1 agenda-db-raft-node-2 agenda-db-raft-node-3 $REDIS_SUPERVISOR_NAME $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME 2>/dev/null
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
    echo "Redis Cluster:"
    docker ps --filter "name=agenda-redis-" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""
    echo "Redis Supervisor:"
    docker ps --filter "name=agenda-redis-supervisor-service" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""
    echo "Raft DB Cluster:"
    docker ps --filter "name=agenda-db-raft-node" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""
    echo "User Service:"
    docker ps --filter "name=agenda-user-service" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""
    echo "Group Service:"
    docker ps --filter "name=agenda-group-service" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""
    echo "=== Redis Roles ==="
    echo "Redis A:"
    docker exec $REDIS_A_NAME redis-cli INFO replication 2>/dev/null | grep role || echo "Not responding"
    echo "Redis B:"
    docker exec $REDIS_B_NAME redis-cli INFO replication 2>/dev/null | grep role || echo "Not responding"
    echo "Redis C:"
    docker exec $REDIS_C_NAME redis-cli INFO replication 2>/dev/null | grep role || echo "Not responding"
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
        echo "Starting all services in order: redis cluster + supervisor → raft-db → user → group"
        clean_data
        start_redis
        sleep 5  # Give Redis cluster + supervisor time to initialize
        start_raft_db
        sleep 5
        start_user
        sleep 2
        start_group
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
    group)
        start_group
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
        echo "Available services: all, redis, raft-db, user, group, stop, clean, status"
        exit 1
        ;;
esac

echo ""
echo "=== Useful Commands ==="
echo "Check Raft status: curl http://localhost:8001/raft/status"
echo "Check Redis roles: docker exec $REDIS_A_NAME redis-cli INFO replication | grep role"
echo "Check Redis Supervisor logs: docker logs $REDIS_SUPERVISOR_NAME"
echo "Check User Service logs: docker logs agenda-user-service"
echo "Check Group Service logs: docker logs agenda-group-service"
echo "Test User Service: curl http://localhost:8004/health"
echo "Test Group Service: curl http://localhost:8005/health"
echo "Check Raft logs: docker logs agenda-db-raft-node-1"
echo "Simulate Redis failover: docker stop $REDIS_A_NAME"
