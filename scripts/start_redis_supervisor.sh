#!/bin/bash

# Exit on any error
set -e

NETWORK_NAME="agenda-network"
REDIS_A_NAME="agenda-redis-a-service"
REDIS_B_NAME="agenda-redis-b-service"
REDIS_SUPERVISOR_NAME="agenda-redis-supervisor-service"
DB_SERVICE_NAME="agenda-db-service" # Assuming DB service is running or will be started separately

echo "--- Stopping and removing existing containers ---"
docker stop $REDIS_SUPERVISOR_NAME > /dev/null 2>&1 || true
docker rm $REDIS_SUPERVISOR_NAME > /dev/null 2>&1 || true
docker stop $REDIS_A_NAME > /dev/null 2>&1 || true
docker rm $REDIS_A_NAME > /dev/null 2>&1 || true
docker stop $REDIS_B_NAME > /dev/null 2>&1 || true
docker rm $REDIS_B_NAME > /dev/null 2>&1 || true

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

echo "--- Starting Redis Supervisor Service ---"
docker run -d --name $REDIS_SUPERVISOR_NAME --network $NETWORK_NAME \
  -e REDIS_ADDRS="${REDIS_A_NAME}:6379,${REDIS_B_NAME}:6379" \
  -e DB_SERVICE_URL="http://${DB_SERVICE_NAME}:8000" \
  -e PING_INTERVAL=1 \
  -e FAILURE_THRESHOLD=3 \
  agenda-redis-supervisor
echo "Redis Supervisor Service started"

echo "\n--- Test Instructions ---"
echo "1. Ensure agenda-db-service is running: ./scripts/start.sh db"
echo "2. Build the supervisor: ./scripts/build.sh redis-supervisor"
echo "3. Run this script: ./scripts/start_redis_supervisor.sh"
echo "4. Check supervisor logs for initial primary detection: docker logs -f $REDIS_SUPERVISOR_NAME"
echo "5. Verify Redis roles: docker exec $REDIS_A_NAME redis-cli INFO replication | grep role"
echo "                   docker exec $REDIS_B_NAME redis-cli INFO replication | grep role"
echo "6. To simulate a failure, stop Redis A: docker stop $REDIS_A_NAME"
echo "7. Observe supervisor logs for failover. Redis B should become master: docker logs -f $REDIS_SUPERVISOR_NAME"
echo "8. Verify new Redis roles: docker exec $REDIS_B_NAME redis-cli INFO replication | grep role"
echo "9. Check the DB service config entry (requires DB service endpoint for config lookup)."
