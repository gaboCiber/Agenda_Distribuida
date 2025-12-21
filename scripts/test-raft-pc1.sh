#!/bin/bash

# Docker network name
NETWORK_NAME="agenda-test"

# Get absolute path of the current directory
CURRENT_DIR="$(pwd)"

# Check if service name is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 [all|redis|raft-db|user|group|api|stop|clean|status|redis-status]"
    echo "  all       - Start all PC1 services in order"
    echo "  redis     - Start Redis cluster + supervisor (PC1 nodes only)"
    echo "  raft-db   - Start Raft DB cluster (PC1 nodes only)"
    echo "  user      - Start User Service only"
    echo "  group     - Start Group Service only"
    echo "  api       - Start API Gateway only"
    echo "  stop      - Stop all PC1 services"
    echo "  clean     - Stop services and clean data"
    echo "  status    - Show status of PC1 services"
    echo "  redis-status - Show Redis roles for all nodes (PC1 and PC2)"
    echo ""
    echo "Note: This script runs PC1 services only. Run test-raft-pc2.sh on PC2."
    exit 1
fi

SERVICE=$1

# Create the network if it doesn't exist
echo "Checking Docker network..."
docker network inspect $NETWORK_NAME >/dev/null 2>&1 || \
    docker network create --driver bridge $NETWORK_NAME

# Configuration for PC1
PC1_IP="localhost"
PC2_IP="192.168.1.104"

# Redis cluster configuration for PC1 (3 nodes)
REDIS_A_NAME="agenda-redis-a-service"
REDIS_B_NAME="agenda-redis-b-service" 
REDIS_C_NAME="agenda-redis-c-service"
REDIS_D_NAME="agenda-redis-d-service"
REDIS_E_NAME="agenda-redis-e-service" 
REDIS_F_NAME="agenda-redis-f-service"

REDIS_SUPERVISOR_1_NAME="agenda-redis-supervisor-sup-1"
REDIS_SUPERVISOR_2_NAME="agenda-redis-supervisor-sup-2"
REDIS_SUPERVISOR_3_NAME="agenda-redis-supervisor-sup-3"
REDIS_SUPERVISOR_4_NAME="agenda-redis-supervisor-sup-4"
REDIS_SUPERVISOR_5_NAME="agenda-redis-supervisor-sup-5"
REDIS_SUPERVISOR_6_NAME="agenda-redis-supervisor-sup-6"

PEERS_LIST="sup-1=${REDIS_SUPERVISOR_1_NAME}:6001,sup-2=${REDIS_SUPERVISOR_2_NAME}:6002,sup-3=${REDIS_SUPERVISOR_3_NAME}:6003,sup-4=${REDIS_SUPERVISOR_4_NAME}:6004,sup-5=${REDIS_SUPERVISOR_5_NAME}:6005,sup-6=${REDIS_SUPERVISOR_6_NAME}:6006"

start_redis() {
    echo "Starting Redis cluster + supervisor (PC1 nodes)..."
    
    # Stop existing Redis containers
    echo "Stopping existing Redis containers..."
    docker stop $REDIS_SUPERVISOR_1_NAME $REDIS_SUPERVISOR_2_NAME $REDIS_SUPERVISOR_3_NAME $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME > /dev/null 2>&1 || true
    docker rm $REDIS_SUPERVISOR_1_NAME $REDIS_SUPERVISOR_2_NAME $REDIS_SUPERVISOR_3_NAME $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME > /dev/null 2>&1 || true

    # Start Redis A (Master)
    echo "Starting Redis A (Master)..."
    docker run -d --name $REDIS_A_NAME --network $NETWORK_NAME \
      -p 6379:6379 \
      redis:7-alpine
    echo "Redis A (Master) started on port 6379"
    
    sleep 2
    
    # Start Redis B (Master initially)
    echo "Starting Redis B (Master initially)..."
    docker run -d --name $REDIS_B_NAME --network $NETWORK_NAME \
      -p 6380:6379 \
      redis:7-alpine
    echo "Redis B (Master initially) started on port 6380"
    
    sleep 2
    
    # Start Redis C (Master initially)
    echo "Starting Redis C (Master initially)..."
    docker run -d --name $REDIS_C_NAME --network $NETWORK_NAME \
      -p 6381:6379 \
      redis:7-alpine
    echo "Redis C (Master initially) started on port 6381"
    
    sleep 2
    
    # Start Redis Supervisors (3 supervisors on PC1)
    echo "Starting Redis Supervisors (PC1)..."
    
    # Supervisor 1
    docker run -d --name $REDIS_SUPERVISOR_1_NAME --network $NETWORK_NAME \
      -p 6001:6001 -p 8080:8080 \
      -e REDIS_ADDRS="${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379,${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379" \
      -e DB_SERVICE_URL="http://agenda-db-raft-node-1:8001" \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006" \
      -e SUPERVISOR_ID=sup-1 \
      -e SUPERVISOR_BIND_ADDR=:6001 \
      -e HTTP_PORT=8080 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor
      
    # Supervisor 2
    docker run -d --name $REDIS_SUPERVISOR_2_NAME --network $NETWORK_NAME \
      -p 6002:6002 -p 8081:8081 \
      -e REDIS_ADDRS="${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379,${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379" \
      -e DB_SERVICE_URL="http://agenda-db-raft-node-1:8001" \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006" \
      -e SUPERVISOR_ID=sup-2 \
      -e SUPERVISOR_BIND_ADDR=:6002 \
      -e HTTP_PORT=8081 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor
      
    # Supervisor 3
    docker run -d --name $REDIS_SUPERVISOR_3_NAME --network $NETWORK_NAME \
      -p 6003:6003 -p 8082:8082 \
      -e REDIS_ADDRS="${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379,${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379" \
      -e DB_SERVICE_URL="http://agenda-db-raft-node-1:8001" \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006" \
      -e SUPERVISOR_ID=sup-3 \
      -e SUPERVISOR_BIND_ADDR=:6003 \
      -e HTTP_PORT=8082 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor
    
    echo "Redis Supervisors started"
    
    echo "=== Redis cluster started (PC1) ==="
    echo "Redis A (Master initially): ${REDIS_A_NAME}:6379"
    echo "Redis B (Master initially): ${REDIS_B_NAME}:6379" 
    echo "Redis C (Master initially): ${REDIS_C_NAME}:6379"
    echo "Redis Supervisors: ${REDIS_SUPERVISOR_1_NAME}, ${REDIS_SUPERVISOR_2_NAME}, ${REDIS_SUPERVISOR_3_NAME}"
    echo "Note: Supervisor will converge cluster to single master automatically"
}

start_raft_db() {
    echo "Starting Raft DB Cluster (PC1 nodes - 3 nodes)..."
    
    # Node 1
    echo "Starting Raft DB Node 1 (potential leader)..."
    docker run -d --name agenda-db-raft-node-1 --network $NETWORK_NAME \
      -p 8001:8001 \
      -e RAFT_ID=node1 \
      -e RAFT_PEERS="node1=agenda-db-raft-node-1:9001,node2=agenda-db-raft-node-2:9002,node3=agenda-db-raft-node-3:9003,node4=agenda-db-raft-node-4:9004,node5=agenda-db-raft-node-5:9005,node6=agenda-db-raft-node-6:9006" \
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
      -e RAFT_PEERS="node1=agenda-db-raft-node-1:9001,node2=agenda-db-raft-node-2:9002,node3=agenda-db-raft-node-3:9003,node4=agenda-db-raft-node-4:9004,node5=agenda-db-raft-node-5:9005,node6=agenda-db-raft-node-6:9006" \
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
      -e RAFT_PEERS="node1=agenda-db-raft-node-1:9001,node2=agenda-db-raft-node-2:9002,node3=agenda-db-raft-node-3:9003,node4=agenda-db-raft-node-4:9004,node5=agenda-db-raft-node-5:9005,node6=agenda-db-raft-node-6:9006" \
      -e RAFT_DATA_DIR=/data/node3/raft \
      -e DB_PATH=/data/node3/app.db \
      -e SERVER_PORT=8003 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e LOG_LEVEL=debug \
      agenda-db_event
    echo "Raft DB Node 3 started at localhost:8003"
    
    echo "Raft DB Cluster started with 3 nodes (PC1)"
    echo "Ports: 8001, 8002, 8003"
}

start_user() {
    echo "Starting User Service..."
    docker run -d --name agenda-user-service --network $NETWORK_NAME \
      -p 8007:8007 \
      -e REDIS_URL=redis://agenda-redis-a-service:6379 \
      -e REDIS_CHANNEL=users_events \
      -e DB_SERVICE_URL=http://agenda-db-raft-node-1:8001 \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003" \
      -e LOG_LEVEL=debug \
      agenda-user_event
    echo "User Service started at localhost:8007"
}

start_group() {
    echo "Starting Group Service..."
    docker run -d --name agenda-group-service --network $NETWORK_NAME \
      -p 8008:8008 \
      -e REDIS_URL=redis://agenda-redis-a-service:6379 \
      -e REDIS_CHANNEL=groups_events \
      -e DB_SERVICE_URL=http://agenda-db-raft-node-1:8001 \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003" \
      -e LOG_LEVEL=debug \
      agenda-group_event
    echo "Group Service started at localhost:8008"
}

start_api() {
    echo "Starting API Gateway..."
    docker run -d --name agenda-api-gateway --network $NETWORK_NAME \
      -p 8070:8080 \
      -e REDIS_URL=redis://agenda-redis-a-service:6379 \
      -e DB_SERVICE_URL=http://agenda-db-raft-node-1:8001 \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003" \
      -e JWT_SECRET="your-secret-key-change-in-production" \
      -e JWT_EXPIRATION="24h" \
      -e LOG_LEVEL=debug \
      agenda-api-gateway
    echo "API Gateway started at localhost:8080"
}

stop_services() {
    echo "Stopping all PC1 services..."
    
    # Stop Redis and supervisors
    docker stop $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME $REDIS_SUPERVISOR_1_NAME $REDIS_SUPERVISOR_2_NAME $REDIS_SUPERVISOR_3_NAME > /dev/null 2>&1 || true
    docker rm $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME $REDIS_SUPERVISOR_1_NAME $REDIS_SUPERVISOR_2_NAME $REDIS_SUPERVISOR_3_NAME > /dev/null 2>&1 || true
    
    # Stop Raft nodes
    docker stop agenda-db-raft-node-1 agenda-db-raft-node-2 agenda-db-raft-node-3 > /dev/null 2>&1 || true
    docker rm agenda-db-raft-node-1 agenda-db-raft-node-2 agenda-db-raft-node-3 > /dev/null 2>&1 || true
    
    # Stop User, Group, and API services
    docker stop agenda-user-service agenda-group-service agenda-api-gateway > /dev/null 2>&1 || true
    docker rm agenda-user-service agenda-group-service agenda-api-gateway > /dev/null 2>&1 || true
    
    echo "All PC1 services stopped"
}

clean_data() {
    echo "Cleaning data directories..."
    rm -rf "$CURRENT_DIR/services/db_service/data/node1"
    rm -rf "$CURRENT_DIR/services/db_service/data/node2"
    rm -rf "$CURRENT_DIR/services/db_service/data/node3"
    echo "PC1 data directories cleaned"
}

show_status() {
    echo "=== PC1 Service Status ==="
    echo "Redis Cluster:"
    echo "  Redis A:"
    docker ps --filter "name=$REDIS_A_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo "  Redis B:"
    docker ps --filter "name=$REDIS_B_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo "  Redis C:"
    docker ps --filter "name=$REDIS_C_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo ""
    echo "Redis Supervisors:"
    echo "  Supervisor 1:"
    docker ps --filter "name=$REDIS_SUPERVISOR_1_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo "  Supervisor 2:"
    docker ps --filter "name=$REDIS_SUPERVISOR_2_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo "  Supervisor 3:"
    docker ps --filter "name=$REDIS_SUPERVISOR_3_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
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
    echo "API Gateway:"
    docker ps --filter "name=agenda-api-gateway" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
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
    echo ""
    echo "=== Supervisor HTTP Endpoints ==="
    echo "PC1: http://localhost:8080/leader, http://localhost:8081/leader, http://localhost:8082/leader"
    echo ""
    echo "=== Useful Commands ==="
    echo "Test API Gateway: curl http://localhost:8080/"
    echo "Test API registration: curl -X POST http://localhost:8080/api/auth/register -H 'Content-Type: application/json' -d '{\"username\":\"test\",\"email\":\"test@example.com\",\"password\":\"password123\"}'"
}

show_redis_status() {
    echo "=== PC1 Redis Nodes ==="
    echo ""
    echo "Redis A (Master):"
    docker exec $REDIS_A_NAME redis-cli INFO replication 2>/dev/null | grep -E "role|master_host" || echo "    Not responding"
    echo ""
    echo "Redis B (Replica):"
    docker exec $REDIS_B_NAME redis-cli INFO replication 2>/dev/null | grep -E "role|master_host" || echo "    Not responding"
    echo ""
    echo "Redis C (Replica):"
    docker exec $REDIS_C_NAME redis-cli INFO replication 2>/dev/null | grep -E "role|master_host" || echo "    Not responding"
    echo ""
    echo "=== PC2 Redis Nodes (checking remotely) ==="
    echo ""
    echo "Redis D (Replica):"
    ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 $PC2_IP "docker exec agenda-redis-d-service redis-cli INFO replication 2>/dev/null | grep -E 'role|master_host'" 2>/dev/null || echo "    Not responding or PC2 not reachable"
    echo ""
    echo "Redis E (Replica):"
    ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 $PC2_IP "docker exec agenda-redis-e-service redis-cli INFO replication 2>/dev/null | grep -E 'role|master_host'" 2>/dev/null || echo "    Not responding or PC2 not reachable"
    echo ""
    echo "Redis F (Replica):"
    ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 $PC2_IP "docker exec agenda-redis-f-service redis-cli INFO replication 2>/dev/null | grep -E 'role|master_host'" 2>/dev/null || echo "    Not responding or PC2 not reachable"
    echo ""
}

case $SERVICE in
    all)
        echo "Starting all PC1 services in order: raft-db → redis cluster + supervisor → user → group → api"
        start_raft_db
        sleep 5
        clean_data
        start_redis
        sleep 5
        start_user
        sleep 2
        start_group
        sleep 2
        start_api
        sleep 2
        echo ""
        echo "=== All PC1 Services Started ==="
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
    api)
        start_api
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
    redis-status)
        show_redis_status
        ;;
    *)
        echo "Unknown service: $SERVICE"
        echo "Available services: all, redis, raft-db, user, group, api, stop, clean, status, redis-status"
        exit 1
        ;;
esac

echo ""
echo "=== PC1 Deployment Complete ==="
echo "Run test-raft-pc2.sh on PC2 to start the other half of the cluster"
