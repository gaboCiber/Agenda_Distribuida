#!/bin/bash

# Exit on any error
set -e

NETWORK_NAME="agenda-network"
REDIS_A_NAME="agenda-redis-a-service"
REDIS_B_NAME="agenda-redis-b-service"
REDIS_C_NAME="agenda-redis-c-service"
REDIS_SUPERVISOR_PREFIX="agenda-redis-supervisor"
DB_SERVICE_NAME="agenda-db-service" # Assuming DB service is running or will be started separately

# Supervisor configuration
SUPERVISOR_IDS=("sup-1" "sup-2" "sup-3")
SUPERVISOR_PORTS=("6001" "6002" "6003")
SUPERVISOR_COUNT=3

# Function to stop and remove all containers
stop_and_remove_containers() {
    echo "--- Stopping and removing existing containers ---"
    
    # Stop Redis containers
    docker stop $REDIS_A_NAME > /dev/null 2>&1 || true
    docker rm $REDIS_A_NAME > /dev/null 2>&1 || true
    docker stop $REDIS_B_NAME > /dev/null 2>&1 || true
    docker rm $REDIS_B_NAME > /dev/null 2>&1 || true
    docker stop $REDIS_C_NAME > /dev/null 2>&1 || true
    docker rm $REDIS_C_NAME > /dev/null 2>&1 || true

    # Stop Supervisor containers
    for i in "${!SUPERVISOR_IDS[@]}"; do
        SUPERVISOR_NAME="${REDIS_SUPERVISOR_PREFIX}-${SUPERVISOR_IDS[$i]}"
        docker stop $SUPERVISOR_NAME > /dev/null 2>&1 || true
        docker rm $SUPERVISOR_NAME > /dev/null 2>&1 || true
    done
    
    echo "All containers stopped and removed."
}

# Check for command line arguments
if [ "$1" = "stop" ]; then
    stop_and_remove_containers
    exit 0
fi

if [ "$1" = "restart" ]; then
    stop_and_remove_containers
    echo "Restarting containers..."
    # Continue with the normal startup process
fi

echo "--- Checking Docker network ---"
docker network inspect $NETWORK_NAME >/dev/null 2>&1 || \
    docker network create --driver bridge $NETWORK_NAME

echo "--- Starting Redis A (Master) ---"
docker run -d --name $REDIS_A_NAME --network $NETWORK_NAME \
  -p 6379:6379 \
  redis:7-alpine
echo "Redis A (Master) started on port 6379"

sleep 2

echo "--- Starting Redis B (Replica of Redis A) ---"
docker run -d --name $REDIS_B_NAME --network $NETWORK_NAME \
  -p 6380:6379 \
  redis:7-alpine redis-server --replicaof $REDIS_A_NAME 6379
echo "Redis B (Replica) started on port 6380"

sleep 2

echo "--- Starting Redis C (Replica of Redis A) ---"
docker run -d --name $REDIS_C_NAME --network $NETWORK_NAME \
  -p 6381:6379 \
  redis:7-alpine redis-server --replicaof $REDIS_A_NAME 6379
echo "Redis C (Replica) started on port 6381"

sleep 2

# Build the SUPERVISOR_PEERS string for environment variable
PEERS_LIST=""
for i in "${!SUPERVISOR_IDS[@]}"; do
    ID="${SUPERVISOR_IDS[$i]}"
    ADDR="${REDIS_SUPERVISOR_PREFIX}-${ID}:${SUPERVISOR_PORTS[$i]}"
    if [ -n "$PEERS_LIST" ]; then
        PEERS_LIST="${PEERS_LIST},"
    fi
    PEERS_LIST="${PEERS_LIST}${ID}=${ADDR}"
done

echo "--- Starting Redis Supervisor Cluster (${SUPERVISOR_COUNT} instances) ---"
for i in "${!SUPERVISOR_IDS[@]}"; do
    ID="${SUPERVISOR_IDS[$i]}"
    PORT="${SUPERVISOR_PORTS[$i]}"
    SUPERVISOR_NAME="${REDIS_SUPERVISOR_PREFIX}-${ID}"

    echo "Starting ${SUPERVISOR_NAME} (ID: ${ID}, gRPC port: ${PORT}, HTTP port: $((8080 + i)) )..."
    docker run -d --name $SUPERVISOR_NAME --network $NETWORK_NAME \
      -p "${PORT}:${PORT}" \
      -p "$((8080 + i)):$((8080 + i))" \
      -e REDIS_ADDRS="${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379,${REDIS_C_NAME}:6379" \
      -e DB_SERVICE_URL="http://${DB_SERVICE_NAME}:8001" \
      -e RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003" \
      -e PING_INTERVAL=1 \
      -e FAILURE_THRESHOLD=3 \
      -e SUPERVISOR_ID="${ID}" \
      -e SUPERVISOR_BIND_ADDR=":${PORT}" \
      -e SUPERVISOR_PEERS="${PEERS_LIST}" \
      -e HTTP_PORT="$((8080 + i))" \
      agenda-redis-supervisor
done

echo "Redis Supervisor Cluster started"

echo "=== Roles actuales ==="
echo "Redis A:"
docker exec agenda-redis-a-service redis-cli INFO replication | grep role
echo "Redis B:"
docker exec agenda-redis-b-service redis-cli INFO replication | grep role
echo "Redis C:"
docker exec agenda-redis-c-service redis-cli INFO replication | grep role

echo ""
echo "=== Supervisor Instances ==="
for i in "${!SUPERVISOR_IDS[@]}"; do
    SUPERVISOR_NAME="${REDIS_SUPERVISOR_PREFIX}-${SUPERVISOR_IDS[$i]}"
    HTTP_PORT=$((8080 + i))
    echo "${SUPERVISOR_NAME} (gRPC port: ${SUPERVISOR_PORTS[$i]}, HTTP port: ${HTTP_PORT})"
done

echo ""
echo "--- Test Instructions ---"
echo "Usage:"
echo "  ./scripts/start_redis_supervisor_cluster.sh          - Start all containers"
echo "  ./scripts/start_redis_supervisor_cluster.sh stop     - Stop and remove all containers"
echo "  ./scripts/start_redis_supervisor_cluster.sh restart  - Restart all containers"
echo ""
echo "1. Ensure agenda-db-service is running: ./scripts/start.sh db"
echo "2. Build the supervisor: ./scripts/build.sh redis-supervisor"
echo "3. Run this script: ./scripts/start_redis_supervisor_cluster.sh"
echo "4. Check supervisor logs to see leader election: docker logs -f ${REDIS_SUPERVISOR_PREFIX}-${SUPERVISOR_IDS[0]}"
echo "5. Verify which supervisor is the leader by checking logs of all instances:"
for i in "${!SUPERVISOR_IDS[@]}"; do
    echo "   docker logs ${REDIS_SUPERVISOR_PREFIX}-${SUPERVISOR_IDS[$i]}"
done
echo "6. Verify Redis roles: docker exec $REDIS_A_NAME redis-cli INFO replication | grep role"
echo "7. To simulate a failure, stop the leader supervisor: docker stop ${REDIS_SUPERVISOR_PREFIX}-<LEADER_ID>"
echo "8. Observe supervisor logs for new leader election."
echo "9. To simulate a Redis failure, stop Redis A: docker stop $REDIS_A_NAME"
echo "10. Observe failover in the current leader supervisor's logs."
echo "11. Verify new Redis roles: docker exec $REDIS_B_NAME redis-cli INFO replication | grep role"
echo "12. Check the DB service config entry (requires DB service endpoint for config lookup)."
echo "13. Test multiple supervisor failures: stop multiple supervisor instances and observe leader election."
echo "14. To test partition tolerance, you can use network tools to simulate network partitions between supervisor instances."
echo "15. Check the HTTP endpoint for leader info:"
for i in "${!SUPERVISOR_IDS[@]}"; do
    HTTP_PORT=$((8080 + i))
    echo "   curl http://localhost:${HTTP_PORT}/leader"
done
