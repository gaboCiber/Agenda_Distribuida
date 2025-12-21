#!/bin/bash

# Docker network name
NETWORK_NAME="agenda-test"

# Get absolute path of the current directory
CURRENT_DIR="$(pwd)"

# Check if service name is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 [all|redis|raft-db|stop|clean|status|redis-status]"
    echo "  all       - Start all PC2 services in order"
    echo "  redis     - Start Redis cluster + supervisor (PC2 nodes only)"
    echo "  raft-db   - Start Raft DB cluster (PC2 nodes only)"
    echo "  stop      - Stop all PC2 services"
    echo "  clean     - Stop services and clean data"
    echo "  status    - Show status of PC2 services"
    echo "  redis-status - Show Redis roles for all nodes (PC1 and PC2)"
    echo ""
    echo "Note: This script runs PC2 services only. Run test-raft-pc1.sh on PC1."
    exit 1
fi

SERVICE=$1

# Create the network if it doesn't exist
echo "Checking Docker network..."
docker network inspect $NETWORK_NAME >/dev/null 2>&1 || \
    docker network create --driver bridge $NETWORK_NAME

# Configuration for PC2
PC1_IP="192.168.1.103"  # IP of PC1
PC2_IP="localhost"      # Current machine

# Redis cluster configuration for PC2 (3 nodes)
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
    echo "Starting Redis cluster + supervisor (PC2 nodes)..."
    
    # Stop existing Redis containers
    echo "Stopping existing Redis containers..."
    docker stop $REDIS_SUPERVISOR_4_NAME $REDIS_SUPERVISOR_5_NAME $REDIS_SUPERVISOR_6_NAME $REDIS_D_NAME $REDIS_E_NAME $REDIS_F_NAME > /dev/null 2>&1 || true
    docker rm $REDIS_SUPERVISOR_4_NAME $REDIS_SUPERVISOR_5_NAME $REDIS_SUPERVISOR_6_NAME $REDIS_D_NAME $REDIS_E_NAME $REDIS_F_NAME > /dev/null 2>&1 || true

    # Start Redis D (Master initially)
    echo "Starting Redis D (Master initially)..."
    docker run -d --name $REDIS_D_NAME --network $NETWORK_NAME \
      -p 6379:6379 \
      redis:7-alpine
    echo "Redis D (Master initially) started on port 6379"
    
    sleep 2
    
    # Start Redis E (Master initially)
    echo "Starting Redis E (Master initially)..."
    docker run -d --name $REDIS_E_NAME --network $NETWORK_NAME \
      -p 6380:6379 \
      redis:7-alpine
    echo "Redis E (Master initially) started on port 6380"
    
    sleep 2
    
    # Start Redis F (Master initially)
    echo "Starting Redis F (Master initially)..."
    docker run -d --name $REDIS_F_NAME --network $NETWORK_NAME \
      -p 6381:6379 \
      redis:7-alpine
    echo "Redis F (Master initially) started on port 6381"
    
    sleep 2
    
    # Start Redis Supervisors (3 supervisors on PC2)
    echo "Starting Redis Supervisors (PC2)..."
    
    # Supervisor 4
    docker run -d --name $REDIS_SUPERVISOR_4_NAME --network $NETWORK_NAME \
      -p 6004:6004 -p 8083:8083 \
      -e REDIS_ADDRS="${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379,${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379" \
      -e DB_SERVICE_URL="http://agenda-db-raft-node-1:8001" \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006" \
      -e SUPERVISOR_ID=sup-4 \
      -e SUPERVISOR_BIND_ADDR=:6004 \
      -e HTTP_PORT=8083 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor
      
    # Supervisor 5
    docker run -d --name $REDIS_SUPERVISOR_5_NAME --network $NETWORK_NAME \
      -p 6005:6005 -p 8084:8084 \
      -e REDIS_ADDRS="${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379,${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379" \
      -e DB_SERVICE_URL="http://agenda-db-raft-node-1:8001" \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006" \
      -e SUPERVISOR_ID=sup-5 \
      -e SUPERVISOR_BIND_ADDR=:6005 \
      -e HTTP_PORT=8084 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor
      
    # Supervisor 6
    docker run -d --name $REDIS_SUPERVISOR_6_NAME --network $NETWORK_NAME \
      -p 6006:6006 -p 8085:8085 \
      -e REDIS_ADDRS="${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379,${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379" \
      -e DB_SERVICE_URL="http://agenda-db-raft-node-1:8001" \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006" \
      -e SUPERVISOR_ID=sup-6 \
      -e SUPERVISOR_BIND_ADDR=:6006 \
      -e HTTP_PORT=8085 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor
    
    echo "Redis Supervisors started"
    
    echo "=== Redis cluster started (PC2) ==="
    echo "Redis D (Master initially): ${REDIS_D_NAME}:6379"
    echo "Redis E (Master initially): ${REDIS_E_NAME}:6379" 
    echo "Redis F (Master initially): ${REDIS_F_NAME}:6379"
    echo "Redis Supervisors: ${REDIS_SUPERVISOR_4_NAME}, ${REDIS_SUPERVISOR_5_NAME}, ${REDIS_SUPERVISOR_6_NAME}"
    echo "Note: Supervisor will converge cluster to single master automatically"
}

start_raft_db() {
    echo "Starting Raft DB Cluster (PC2 nodes - 3 nodes)..."
    
    # Node 4
    echo "Starting Raft DB Node 4..."
    docker run -d --name agenda-db-raft-node-4 --network $NETWORK_NAME \
      -p 8004:8004 \
      -e RAFT_ID=node4 \
      -e RAFT_PEERS="node1=agenda-db-raft-node-1:9001,node2=agenda-db-raft-node-2:9002,node3=agenda-db-raft-node-3:9003,node4=agenda-db-raft-node-4:9004,node5=agenda-db-raft-node-5:9005,node6=agenda-db-raft-node-6:9006" \
      -e RAFT_DATA_DIR=/data/node4/raft \
      -e DB_PATH=/data/node4/app.db \
      -e SERVER_PORT=8004 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e LOG_LEVEL=debug \
      agenda-db_event
    echo "Raft DB Node 4 started at localhost:8004"
    
    # Node 5
    echo "Starting Raft DB Node 5..."
    docker run -d --name agenda-db-raft-node-5 --network $NETWORK_NAME \
      -p 8005:8005 \
      -e RAFT_ID=node5 \
      -e RAFT_PEERS="node1=agenda-db-raft-node-1:9001,node2=agenda-db-raft-node-2:9002,node3=agenda-db-raft-node-3:9003,node4=agenda-db-raft-node-4:9004,node5=agenda-db-raft-node-5:9005,node6=agenda-db-raft-node-6:9006" \
      -e RAFT_DATA_DIR=/data/node5/raft \
      -e DB_PATH=/data/node5/app.db \
      -e SERVER_PORT=8005 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e LOG_LEVEL=debug \
      agenda-db_event
    echo "Raft DB Node 5 started at localhost:8005"
    
    # Node 6
    echo "Starting Raft DB Node 6..."
    docker run -d --name agenda-db-raft-node-6 --network $NETWORK_NAME \
      -p 8006:8006 \
      -e RAFT_ID=node6 \
      -e RAFT_PEERS="node1=agenda-db-raft-node-1:9001,node2=agenda-db-raft-node-2:9002,node3=agenda-db-raft-node-3:9003,node4=agenda-db-raft-node-4:9004,node5=agenda-db-raft-node-5:9005,node6=agenda-db-raft-node-6:9006" \
      -e RAFT_DATA_DIR=/data/node6/raft \
      -e DB_PATH=/data/node6/app.db \
      -e SERVER_PORT=8006 \
      -e REDIS_URL=redis://agenda-redis-service:6379 \
      -e LOG_LEVEL=debug \
      agenda-db_event
    echo "Raft DB Node 6 started at localhost:8006"
    
    echo "Raft DB Cluster started with 3 nodes (PC2)"
    echo "Ports: 8004, 8005, 8006"
}

stop_services() {
    echo "Stopping all PC2 services..."
    
    # Stop Redis and supervisors
    docker stop $REDIS_D_NAME $REDIS_E_NAME $REDIS_F_NAME $REDIS_SUPERVISOR_4_NAME $REDIS_SUPERVISOR_5_NAME $REDIS_SUPERVISOR_6_NAME > /dev/null 2>&1 || true
    docker rm $REDIS_D_NAME $REDIS_E_NAME $REDIS_F_NAME $REDIS_SUPERVISOR_4_NAME $REDIS_SUPERVISOR_5_NAME $REDIS_SUPERVISOR_6_NAME > /dev/null 2>&1 || true
    # Stop Raft nodes
    docker stop agenda-db-raft-node-4 agenda-db-raft-node-5 agenda-db-raft-node-6 > /dev/null 2>&1 || true
    docker rm agenda-db-raft-node-4 agenda-db-raft-node-5 agenda-db-raft-node-6 > /dev/null 2>&1 || true
    
    echo "All PC2 services stopped"
}

clean_data() {
    echo "Cleaning data directories..."
    rm -rf "$CURRENT_DIR/services/db_service/data/node4"
    rm -rf "$CURRENT_DIR/services/db_service/data/node5"
    rm -rf "$CURRENT_DIR/services/db_service/data/node6"
    echo "PC2 data directories cleaned"
}

show_status() {
    echo "=== PC2 Service Status ==="
    echo "Redis Cluster:"
    echo "  Redis D:"
    docker ps --filter "name=$REDIS_D_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo "  Redis E:"
    docker ps --filter "name=$REDIS_E_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo "  Redis F:"
    docker ps --filter "name=$REDIS_F_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo ""
    echo "Redis Supervisors:"
    echo "  Supervisor 4:"
    docker ps --filter "name=$REDIS_SUPERVISOR_4_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo "  Supervisor 5:"
    docker ps --filter "name=$REDIS_SUPERVISOR_5_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo "  Supervisor 6:"
    docker ps --filter "name=$REDIS_SUPERVISOR_6_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo '    Not running'
    echo ""
    echo "Raft DB Cluster:"
    docker ps --filter "name=agenda-db-raft-node" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""
    echo "=== Redis Roles ==="
    echo "Redis D:"
    docker exec $REDIS_D_NAME redis-cli INFO replication 2>/dev/null | grep role || echo "Not responding"
    echo "Redis E:"
    docker exec $REDIS_E_NAME redis-cli INFO replication 2>/dev/null | grep role || echo "Not responding"
    echo "Redis F:"
    docker exec $REDIS_F_NAME redis-cli INFO replication 2>/dev/null | grep role || echo "Not responding"
    echo ""
    echo "=== Raft Status ==="
    echo "Node 4 (8004):"
    curl -s http://localhost:8004/raft/status | jq . 2>/dev/null || echo "Not responding"
    echo ""
    echo "Node 5 (8005):"
    curl -s http://localhost:8005/raft/status | jq . 2>/dev/null || echo "Not responding"
    echo ""
    echo "Node 6 (8006):"
    curl -s http://localhost:8006/raft/status | jq . 2>/dev/null || echo "Not responding"
    echo ""
    echo "=== Supervisor HTTP Endpoints ==="
    echo "PC2: http://localhost:8083/leader, http://localhost:8084/leader, http://localhost:8085/leader"
    echo ""
    echo "=== Useful Commands ==="
    echo "Connect to Redis nodes:"
    echo "  PC2: redis-cli -h localhost -p 6379 (D), redis-cli -h localhost -p 6380 (E), redis-cli -h localhost -p 6381 (F)"
    echo "Check PC1 cluster: curl http://${PC1_IP}:8001/raft/status"
}

show_redis_status() {
    echo "=== PC2 Redis Nodes ==="
    echo ""
    echo "Redis D (Replica):"
    docker exec $REDIS_D_NAME redis-cli INFO replication 2>/dev/null | grep -E "role|master_host" || echo "    Not responding"
    echo ""
    echo "Redis E (Replica):"
    docker exec $REDIS_E_NAME redis-cli INFO replication 2>/dev/null | grep -E "role|master_host" || echo "    Not responding"
    echo ""
    echo "Redis F (Replica):"
    docker exec $REDIS_F_NAME redis-cli INFO replication 2>/dev/null | grep -E "role|master_host" || echo "    Not responding"
    echo ""
    echo "=== PC1 Redis Nodes (checking remotely) ==="
    echo ""
    echo "Redis A (Master):"
    ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 $PC1_IP "docker exec agenda-redis-a-service redis-cli INFO replication 2>/dev/null | grep -E 'role|master_replid|master_repl_offset'" 2>/dev/null || echo "    Not responding or PC1 not reachable"
    echo ""
    echo "Redis B (Replica):"
    ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 $PC1_IP "docker exec agenda-redis-b-service redis-cli INFO replication 2>/dev/null | grep -E 'role|master_host'" 2>/dev/null || echo "    Not responding or PC1 not reachable"
    echo ""
    echo "Redis C (Replica):"
    ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 $PC1_IP "docker exec agenda-redis-c-service redis-cli INFO replication 2>/dev/null | grep -E 'role|master_host'" 2>/dev/null || echo "    Not responding or PC1 not reachable"
    echo ""
}
  

case $SERVICE in
    all)
        echo "Starting all PC2 services in order: raft-db → redis cluster + supervisor"
        start_raft_db
        sleep 5
        clean_data
        start_redis
        sleep 5
        echo ""
        echo "=== All PC2 Services Started ==="
        show_status
        ;;
    redis)
        start_redis
        ;;
    raft-db)
        clean_data
        start_raft_db
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
        echo "Available services: all, redis, raft-db, stop, clean, status, redis-status"
        exit 1
        ;;
esac

echo ""
echo "=== PC2 Deployment Complete ==="
echo "Run test-raft-pc1.sh on PC1 to start the other half of the cluster"
