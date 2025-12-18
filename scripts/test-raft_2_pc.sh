#!/bin/bash

# Docker network name
NETWORK_NAME="agenda-test"

# Get absolute path of the current directory
CURRENT_DIR="$(pwd)"

# Check if service name is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 [all|redis|raft-db|user|group|stop|clean|status|failover]"
    echo "  all       - Start all services in order"
    echo "  redis     - Start Redis cluster + supervisors only (distributed)"
    echo "  raft-db   - Start Raft DB cluster (6 nodes)"
    echo "  user      - Start User Service only"
    echo "  group     - Start Group Service only"
    echo "  stop      - Stop all distributed services"
    echo "  clean     - Stop services and clean data"
    echo "  status    - Show status of all distributed services"
    echo "  failover  - Test failover scenarios"
    echo ""
    echo "Failover usage: $0 failover [redis-a|redis-b|redis-c|redis-d|redis-e|redis-f|sup-1|sup-2|sup-3|sup-4|sup-5|sup-6]"
    exit 1
fi

SERVICE=$1

# Create the network if it doesn't exist
echo "Checking Docker network..."
docker network inspect $NETWORK_NAME >/dev/null 2>&1 || \
    docker network create --driver bridge $NETWORK_NAME

# Configuration for 2 PCs with 6 nodes total
PC1_IP="localhost"  # Current machine
PC2_IP="192.168.1.104"

# Redis cluster configuration for 6 nodes distributed across 2 PCs
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

PEERS_LIST="sup-1=$REDIS_SUPERVISOR_1_NAME:6001,sup-2=$PC1_IP:6002,sup-3=$PC1_IP:6003,sup-4=$PC2_IP:6004,sup-5=$PC2_IP:6005,sup-6=$PC2_IP:6006"

# Function to check if a command should run on remote machine
run_on_pc() {
    local pc_ip=$1
    shift
    local command="$@"
    
    if [ "$pc_ip" = "$PC1_IP" ]; then
        eval "$command"
    else
        ssh -i ~/.ssh/id_script gabo@$pc_ip "$command"
    fi
}

start_redis() {
    echo "Starting distributed Redis cluster with 6 nodes + 6 supervisors..."
    
    # Stop existing containers on all PCs
    echo "Stopping existing Redis containers on all PCs..."
    run_on_pc $PC1_IP "docker stop $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME $REDIS_SUPERVISOR_1_NAME $REDIS_SUPERVISOR_2_NAME $REDIS_SUPERVISOR_3_NAME > /dev/null 2>&1 || true"
    run_on_pc $PC1_IP "docker rm $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME $REDIS_SUPERVISOR_1_NAME $REDIS_SUPERVISOR_2_NAME $REDIS_SUPERVISOR_3_NAME > /dev/null 2>&1 || true"
    
    run_on_pc $PC2_IP "docker stop $REDIS_D_NAME $REDIS_E_NAME $REDIS_F_NAME $REDIS_SUPERVISOR_4_NAME $REDIS_SUPERVISOR_5_NAME $REDIS_SUPERVISOR_6_NAME > /dev/null 2>&1 || true"
    run_on_pc $PC2_IP "docker rm $REDIS_D_NAME $REDIS_E_NAME $REDIS_F_NAME $REDIS_SUPERVISOR_4_NAME $REDIS_SUPERVISOR_5_NAME $REDIS_SUPERVISOR_6_NAME > /dev/null 2>&1 || true"

    # Start Redis A on PC1 (Master)
    echo "Starting Redis A (Master) on PC1..."
    run_on_pc $PC1_IP "docker run -d --name $REDIS_A_NAME --network $NETWORK_NAME \
      -p 6379:6379 \
      redis:7-alpine"
    echo "Redis A (Master) started on PC1 port 6379"
    
    sleep 2
    
    # Start Redis B on PC1 (Replica)
    echo "Starting Redis B (Replica) on PC1..."
    run_on_pc $PC1_IP "docker run -d --name $REDIS_B_NAME --network $NETWORK_NAME \
      -p 6380:6379 \
      redis:7-alpine redis-server --replicaof $REDIS_A_NAME 6379"
    echo "Redis B (Replica) started on PC1 port 6380"
    
    sleep 2
    
    # Start Redis C on PC1 (Replica)
    echo "Starting Redis C (Replica) on PC1..."
    run_on_pc $PC1_IP "docker run -d --name $REDIS_C_NAME --network $NETWORK_NAME \
      -p 6381:6379 \
      redis:7-alpine redis-server --replicaof $REDIS_A_NAME 6379"
    echo "Redis C (Replica) started on PC1 port 6381"
    
    sleep 2
    
    # Start Redis D on PC2 (Replica)
    echo "Starting Redis D (Replica) on PC2..."
    run_on_pc $PC2_IP "docker run -d --name $REDIS_D_NAME --network $NETWORK_NAME \
      -p 6379:6379 \
      redis:7-alpine redis-server --replicaof $REDIS_A_NAME 6379"
    echo "Redis D (Replica) started on PC2 port 6379"
    
    sleep 2
    
    # Start Redis E on PC2 (Replica)
    echo "Starting Redis E (Replica) on PC2..."
    run_on_pc $PC2_IP "docker run -d --name $REDIS_E_NAME --network $NETWORK_NAME \
      -p 6380:6379 \
      redis:7-alpine redis-server --replicaof $REDIS_A_NAME 6379"
    echo "Redis E (Replica) started on PC2 port 6380"
    
    sleep 2
    
    # Start Redis F on PC2 (Replica)
    echo "Starting Redis F (Replica) on PC2..."
    run_on_pc $PC2_IP "docker run -d --name $REDIS_F_NAME --network $NETWORK_NAME \
      -p 6381:6379 \
      redis:7-alpine redis-server --replicaof $REDIS_A_NAME 6379"
    echo "Redis F (Replica) started on PC2 port 6381"
    
    sleep 3
    
    # Start Redis Supervisor 1 on PC1
    echo "Starting Redis Supervisor 1 on PC1..."
    run_on_pc $PC1_IP "docker run -d --name $REDIS_SUPERVISOR_1_NAME --network $NETWORK_NAME \
      -p 6001:6001 -p 8080:8080 \
      -e REDIS_ADDRS=\"${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379,${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379\" \
      -e DB_SERVICE_URL=\"http://agenda-db-raft-node-1:8001\" \
      -e RAFT_NODES_URLS=\"http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006\" \
      -e SUPERVISOR_ID=sup-1 \
      -e SUPERVISOR_BIND_ADDR=:6001 \
      -e HTTP_PORT=8080 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor"
    echo "Redis Supervisor 1 started on PC1 (gRPC:6001, HTTP:8080)"
    
    # Start Redis Supervisor 2 on PC1
    echo "Starting Redis Supervisor 2 on PC1..."
    run_on_pc $PC1_IP "docker run -d --name $REDIS_SUPERVISOR_2_NAME --network $NETWORK_NAME \
      -p 6002:6002 -p 8081:8081 \
      -e REDIS_ADDRS=\"${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379,${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379\" \
      -e DB_SERVICE_URL=\"http://agenda-db-raft-node-1:8001\" \
      -e RAFT_NODES_URLS=\"http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006\" \
      -e SUPERVISOR_ID=sup-2 \
      -e SUPERVISOR_BIND_ADDR=:6002 \
      -e HTTP_PORT=8081 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor"
    echo "Redis Supervisor 2 started on PC1 (gRPC:6002, HTTP:8081)"
    
    # Start Redis Supervisor 3 on PC1
    echo "Starting Redis Supervisor 3 on PC1..."
    run_on_pc $PC1_IP "docker run -d --name $REDIS_SUPERVISOR_3_NAME --network $NETWORK_NAME \
      -p 6003:6003 -p 8082:8082 \
      -e REDIS_ADDRS=\"${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379,${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379\" \
      -e DB_SERVICE_URL=\"http://agenda-db-raft-node-1:8001\" \
      -e RAFT_NODES_URLS=\"http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006\" \
      -e SUPERVISOR_ID=sup-3 \
      -e SUPERVISOR_BIND_ADDR=:6003 \
      -e HTTP_PORT=8082 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor"
    echo "Redis Supervisor 3 started on PC1 (gRPC:6003, HTTP:8082)"
    
    # Start Redis Supervisor 4 on PC2
    echo "Starting Redis Supervisor 4 on PC2..."
    run_on_pc $PC2_IP "docker run -d --name $REDIS_SUPERVISOR_4_NAME --network $NETWORK_NAME \
      -p 6004:6004 -p 8083:8083 \
      -e REDIS_ADDRS=\"${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379,${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379\" \
      -e DB_SERVICE_URL=\"http://agenda-db-raft-node-1:8001\" \
      -e RAFT_NODES_URLS=\"http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006\" \
      -e SUPERVISOR_ID=sup-4 \
      -e SUPERVISOR_BIND_ADDR=:6004 \
      -e HTTP_PORT=8083 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor"
    echo "Redis Supervisor 4 started on PC2 (gRPC:6004, HTTP:8083)"
    
    # Start Redis Supervisor 5 on PC2
    echo "Starting Redis Supervisor 5 on PC2..."
    run_on_pc $PC2_IP "docker run -d --name $REDIS_SUPERVISOR_5_NAME --network $NETWORK_NAME \
      -p 6005:6005 -p 8084:8084 \
      -e REDIS_ADDRS=\"${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379,${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379\" \
      -e DB_SERVICE_URL=\"http://agenda-db-raft-node-1:8001\" \
      -e RAFT_NODES_URLS=\"http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006\" \
      -e SUPERVISOR_ID=sup-5 \
      -e SUPERVISOR_BIND_ADDR=:6005 \
      -e HTTP_PORT=8084 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor"
    echo "Redis Supervisor 5 started on PC2 (gRPC:6005, HTTP:8084)"
    
    # Start Redis Supervisor 6 on PC2
    echo "Starting Redis Supervisor 6 on PC2..."
    run_on_pc $PC2_IP "docker run -d --name $REDIS_SUPERVISOR_6_NAME --network $NETWORK_NAME \
      -p 6006:6006 -p 8085:8085 \
      -e REDIS_ADDRS=\"${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379,${REDIS_D_NAME}:6379,${REDIS_E_NAME}:6379,${REDIS_F_NAME}:6379\" \
      -e DB_SERVICE_URL=\"http://agenda-db-raft-node-1:8001\" \
      -e RAFT_NODES_URLS=\"http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006\" \
      -e SUPERVISOR_ID=sup-6 \
      -e SUPERVISOR_BIND_ADDR=:6006 \
      -e HTTP_PORT=8085 \
      -e SUPERVISOR_PEERS=$PEERS_LIST \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      agenda-redis-supervisor"
    echo "Redis Supervisor 6 started on PC2 (gRPC:6006, HTTP:8085)"
    
    echo ""
    echo "=== Distributed Redis cluster with 6 nodes started ==="
    echo "PC1: Redis A (Master) + Redis B,C (Replicas) + Supervisors 1,2,3"
    echo "PC2: Redis D,E,F (Replicas) + Supervisors 4,5,6"
    echo ""
    echo "=== Access Points ==="
    echo "PC1 Redis nodes: ${PC1_IP}:6379 (A), ${PC1_IP}:6380 (B), ${PC1_IP}:6381 (C)"
    echo "PC2 Redis nodes: ${PC2_IP}:6379 (D), ${PC2_IP}:6380 (E), ${PC2_IP}:6381 (F)"
    echo "PC1 Supervisors HTTP: http://${PC1_IP}:8080, http://${PC1_IP}:8081, http://${PC1_IP}:8082"
    echo "PC2 Supervisors HTTP: http://${PC2_IP}:8083, http://${PC2_IP}:8084, http://${PC2_IP}:8085"
}

start_raft_db() {
    echo "Starting Raft DB Cluster (6 nodes)..."
    
    # Node 1 - Leader
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
    
    # Node 4
    echo "Starting Raft DB Node 4..."
    ssh -i ~/.ssh/id_script gabo@192.168.1.104 docker run -d --name agenda-db-raft-node-4 --network $NETWORK_NAME \
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
    ssh -i ~/.ssh/id_script gabo@192.168.1.104 docker run -d --name agenda-db-raft-node-5 --network $NETWORK_NAME \
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
    ssh -i ~/.ssh/id_script gabo@192.168.1.104 docker run -d --name agenda-db-raft-node-6 --network $NETWORK_NAME \
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
    
    echo "Raft DB Cluster started with 6 nodes"
    echo "Ports: 8001, 8002, 8003, 8004, 8005, 8006"
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

stop_services() {
    echo "Stopping all distributed services..."
    
    # Stop Redis and supervisors on all PCs
    run_on_pc $PC1_IP "docker stop $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME $REDIS_SUPERVISOR_1_NAME $REDIS_SUPERVISOR_2_NAME $REDIS_SUPERVISOR_3_NAME > /dev/null 2>&1 || true"
    run_on_pc $PC2_IP "docker stop $REDIS_D_NAME $REDIS_E_NAME $REDIS_F_NAME $REDIS_SUPERVISOR_4_NAME $REDIS_SUPERVISOR_5_NAME $REDIS_SUPERVISOR_6_NAME > /dev/null 2>&1 || true"
    
    # Stop Raft nodes (nodes 1-3 on PC1, 4-6 on PC2)
    run_on_pc $PC1_IP "docker stop agenda-db-raft-node-1 agenda-db-raft-node-2 agenda-db-raft-node-3 > /dev/null 2>&1 || true"
    run_on_pc $PC2_IP "docker stop agenda-db-raft-node-4 agenda-db-raft-node-5 agenda-db-raft-node-6 > /dev/null 2>&1 || true"
    
    # Stop user and group services on PC1
    run_on_pc $PC1_IP "docker stop agenda-user-service agenda-group-service > /dev/null 2>&1 || true"
    
    echo "All distributed services stopped"
}

remove_services(){
    
    echo "Removing all distributed services..."
    
    # Stop Redis and supervisors on all PCs
    run_on_pc $PC1_IP "docker rm $REDIS_A_NAME $REDIS_B_NAME $REDIS_C_NAME $REDIS_SUPERVISOR_1_NAME $REDIS_SUPERVISOR_2_NAME $REDIS_SUPERVISOR_3_NAME > /dev/null 2>&1 || true"
    run_on_pc $PC2_IP "docker rm $REDIS_D_NAME $REDIS_E_NAME $REDIS_F_NAME $REDIS_SUPERVISOR_4_NAME $REDIS_SUPERVISOR_5_NAME $REDIS_SUPERVISOR_6_NAME > /dev/null 2>&1 || true"
    
    # Stop Raft nodes (nodes 1-3 on PC1, 4-6 on PC2)
    run_on_pc $PC1_IP "docker rm agenda-db-raft-node-1 agenda-db-raft-node-2 agenda-db-raft-node-3 > /dev/null 2>&1 || true"
    run_on_pc $PC2_IP "docker rm agenda-db-raft-node-4 agenda-db-raft-node-5 agenda-db-raft-node-6 > /dev/null 2>&1 || true"
    
    # Stop user and group services on PC1
    run_on_pc $PC1_IP "docker rm agenda-user-service agenda-group-service > /dev/null 2>&1 || true"
    
    echo "All distributed services rm"

}

clean_data() {
    echo "Cleaning data directories..."
    rm -rf "$CURRENT_DIR/services/db_service/data/node1"
    rm -rf "$CURRENT_DIR/services/db_service/data/node2"
    rm -rf "$CURRENT_DIR/services/db_service/data/node3"
    rm -rf "$CURRENT_DIR/services/db_service/data/node4"
    rm -rf "$CURRENT_DIR/services/db_service/data/node5"
    rm -rf "$CURRENT_DIR/services/db_service/data/node6"
    echo "Data directories cleaned"
}

show_status() {
    echo "=== Distributed Service Status (6 nodes) ==="
    echo "Redis Cluster:"
    echo "PC1:"
    echo "  Redis A:"
    run_on_pc $PC1_IP "docker ps --filter 'name=$REDIS_A_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo "  Redis B:"
    run_on_pc $PC1_IP "docker ps --filter 'name=$REDIS_B_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo "  Redis C:"
    run_on_pc $PC1_IP "docker ps --filter 'name=$REDIS_C_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo "PC2:"
    echo "  Redis D:"
    run_on_pc $PC2_IP "docker ps --filter 'name=$REDIS_D_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo "  Redis E:"
    run_on_pc $PC2_IP "docker ps --filter 'name=$REDIS_E_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo "  Redis F:"
    run_on_pc $PC2_IP "docker ps --filter 'name=$REDIS_F_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo ""
    echo "Redis Supervisors:"
    echo "PC1:"
    echo "  Supervisor 1:"
    run_on_pc $PC1_IP "docker ps --filter 'name=$REDIS_SUPERVISOR_1_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo "  Supervisor 2:"
    run_on_pc $PC1_IP "docker ps --filter 'name=$REDIS_SUPERVISOR_2_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo "  Supervisor 3:"
    run_on_pc $PC1_IP "docker ps --filter 'name=$REDIS_SUPERVISOR_3_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo "PC2:"
    echo "  Supervisor 4:"
    run_on_pc $PC2_IP "docker ps --filter 'name=$REDIS_SUPERVISOR_4_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo "  Supervisor 5:"
    run_on_pc $PC2_IP "docker ps --filter 'name=$REDIS_SUPERVISOR_5_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo "  Supervisor 6:"
    run_on_pc $PC2_IP "docker ps --filter 'name=$REDIS_SUPERVISOR_6_NAME' --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo '    Not running'"
    echo ""
    echo "=== Redis Roles ==="
    echo "Redis A (PC1):"
    run_on_pc $PC1_IP "docker exec $REDIS_A_NAME redis-cli INFO replication 2>/dev/null | grep role || echo '  Not responding'"
    echo "Redis B (PC1):"
    run_on_pc $PC1_IP "docker exec $REDIS_B_NAME redis-cli INFO replication 2>/dev/null | grep role || echo '  Not responding'"
    echo "Redis C (PC1):"
    run_on_pc $PC1_IP "docker exec $REDIS_C_NAME redis-cli INFO replication 2>/dev/null | grep role || echo '  Not responding'"
    echo "Redis D (PC2):"
    run_on_pc $PC2_IP "docker exec $REDIS_D_NAME redis-cli INFO replication 2>/dev/null | grep role || echo '  Not responding'"
    echo "Redis E (PC2):"
    run_on_pc $PC2_IP "docker exec $REDIS_E_NAME redis-cli INFO replication 2>/dev/null | grep role || echo '  Not responding'"
    echo "Redis F (PC2):"
    run_on_pc $PC2_IP "docker exec $REDIS_F_NAME redis-cli INFO replication 2>/dev/null | grep role || echo '  Not responding'"
    echo ""
    echo "=== Supervisor HTTP Endpoints ==="
    echo "PC1: http://${PC1_IP}:8080/leader, http://${PC1_IP}:8081/leader, http://${PC1_IP}:8082/leader"
    echo "PC2: http://${PC2_IP}:8083/leader, http://${PC2_IP}:8084/leader, http://${PC2_IP}:8085/leader"
    echo ""
    echo "=== Useful Commands ==="
    echo "Check all Redis roles: ./test-raft_2_pc.sh status"
    echo "Connect to Redis nodes:"
    echo "  PC1: redis-cli -h ${PC1_IP} -p 6379 (A), redis-cli -h ${PC1_IP} -p 6380 (B), redis-cli -h ${PC1_IP} -p 6381 (C)"
    echo "  PC2: redis-cli -h ${PC2_IP} -p 6379 (D), redis-cli -h ${PC2_IP} -p 6380 (E), redis-cli -h ${PC2_IP} -p 6381 (F)"
}

test_failover() {
    local service=$1
    case $service in
        redis-a)
            echo "Testing Redis A failover..."
            run_on_pc $PC1_IP "docker stop $REDIS_A_NAME"
            echo "Redis A stopped. Check supervisor logs for failover..."
            ;;
        redis-b)
            echo "Testing Redis B failover..."
            run_on_pc $PC1_IP "docker stop $REDIS_B_NAME"
            echo "Redis B stopped. Check supervisor logs for failover..."
            ;;
        redis-c)
            echo "Testing Redis C failover..."
            run_on_pc $PC1_IP "docker stop $REDIS_C_NAME"
            echo "Redis C stopped. Check supervisor logs for failover..."
            ;;
        redis-d)
            echo "Testing Redis D failover..."
            run_on_pc $PC2_IP "docker stop $REDIS_D_NAME"
            echo "Redis D stopped. Check supervisor logs for failover..."
            ;;
        redis-e)
            echo "Testing Redis E failover..."
            run_on_pc $PC2_IP "docker stop $REDIS_E_NAME"
            echo "Redis E stopped. Check supervisor logs for failover..."
            ;;
        redis-f)
            echo "Testing Redis F failover..."
            run_on_pc $PC2_IP "docker stop $REDIS_F_NAME"
            echo "Redis F stopped. Check supervisor logs for failover..."
            ;;
        sup-1)
            echo "Testing Supervisor 1 failover..."
            run_on_pc $PC1_IP "docker stop $REDIS_SUPERVISOR_1_NAME"
            echo "Supervisor 1 stopped. Check for new leader election..."
            ;;
        sup-2)
            echo "Testing Supervisor 2 failover..."
            run_on_pc $PC1_IP "docker stop $REDIS_SUPERVISOR_2_NAME"
            echo "Supervisor 2 stopped. Check for new leader election..."
            ;;
        sup-3)
            echo "Testing Supervisor 3 failover..."
            run_on_pc $PC1_IP "docker stop $REDIS_SUPERVISOR_3_NAME"
            echo "Supervisor 3 stopped. Check for new leader election..."
            ;;
        sup-4)
            echo "Testing Supervisor 4 failover..."
            run_on_pc $PC2_IP "docker stop $REDIS_SUPERVISOR_4_NAME"
            echo "Supervisor 4 stopped. Check for new leader election..."
            ;;
        sup-5)
            echo "Testing Supervisor 5 failover..."
            run_on_pc $PC2_IP "docker stop $REDIS_SUPERVISOR_5_NAME"
            echo "Supervisor 5 stopped. Check for new leader election..."
            ;;
        sup-6)
            echo "Testing Supervisor 6 failover..."
            run_on_pc $PC2_IP "docker stop $REDIS_SUPERVISOR_6_NAME"
            echo "Supervisor 6 stopped. Check for new leader election..."
            ;;
        *)
            echo "Usage: $0 failover [redis-a|redis-b|redis-c|redis-d|redis-e|redis-f|sup-1|sup-2|sup-3|sup-4|sup-5|sup-6]"
            exit 1
            ;;
    esac
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
    remove)
        stop_services
        remove_services
        ;;
    clean)
        stop_services
        clean_data
        ;;
    status)
        show_status
        ;;
    failover)
        if [ $# -lt 2 ]; then
            echo "Usage: $0 failover [redis-a|redis-b|redis-c|redis-d|redis-e|redis-f|sup-1|sup-2|sup-3|sup-4|sup-5|sup-6]"
            exit 1
        fi
        test_failover $2
        ;;
    *)
        echo "Unknown service: $SERVICE"
        echo "Available services: all, redis, raft-db, user, group, stop, clean, status, failover"
        exit 1
        ;;
esac

# echo ""
# echo "=== Distributed Deployment Commands (6 nodes) ==="
# echo "Check Supervisor leader status:"
# echo "  PC1: curl http://${PC1_IP}:8080/leader, http://${PC1_IP}:8081/leader, http://${PC1_IP}:8082/leader"
# echo "  PC2: curl http://${PC2_IP}:8083/leader, http://${PC2_IP}:8084/leader, http://${PC2_IP}:8085/leader"
# echo ""
# echo "Test Redis failover:"
# echo "  PC1: ./test-raft_2_pc.sh failover redis-a|redis-b|redis-c"
# echo "  PC2: ./test-raft_2_pc.sh failover redis-d|redis-e|redis-f"
# echo ""
# echo "Test Supervisor failover:"
# echo "  PC1: ./test-raft_2_pc.sh failover sup-1|sup-2|sup-3"
# echo "  PC2: ./test-raft_2_pc.sh failover sup-4|sup-5|sup-6"
# echo ""
# echo "Check all Redis roles: ./test-raft_2_pc.sh status"
# echo "Connect to Redis nodes:"
# echo "  PC1: redis-cli -h ${PC1_IP} -p 6379 (A), redis-cli -h ${PC1_IP} -p 6380 (B), redis-cli -h ${PC1_IP} -p 6381 (C)"
# echo "  PC2: redis-cli -h ${PC2_IP} -p 6379 (D), redis-cli -h ${PC2_IP} -p 6380 (E), redis-cli -h ${PC2_IP} -p 6381 (F)"
