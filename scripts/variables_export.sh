#!/bin/bash

# Export variables temporarily for the current session
export_redis_vars() {
    # Docker network name
    export NETWORK_NAME="agenda-test"
    export CURRENT_DIR="$(pwd)"

    # PC Configuration
    export PC1_IP="localhost"
    export PC2_IP="192.168.1.104"

    # Redis service names
    export REDIS_A_NAME="agenda-redis-a-service"
    export REDIS_B_NAME="agenda-redis-b-service" 
    export REDIS_C_NAME="agenda-redis-c-service"
    export REDIS_D_NAME="agenda-redis-d-service"
    export REDIS_E_NAME="agenda-redis-e-service" 
    export REDIS_F_NAME="agenda-redis-f-service"

    # Redis supervisor service names
    export REDIS_SUPERVISOR_1_NAME="agenda-redis-supervisor-sup-1"
    export REDIS_SUPERVISOR_2_NAME="agenda-redis-supervisor-sup-2" 
    export REDIS_SUPERVISOR_3_NAME="agenda-redis-supervisor-sup-3"
    export REDIS_SUPERVISOR_4_NAME="agenda-redis-supervisor-sup-4"
    export REDIS_SUPERVISOR_5_NAME="agenda-redis-supervisor-sup-5"
    export REDIS_SUPERVISOR_6_NAME="agenda-redis-supervisor-sup-6"

    # Peers and Raft configuration
    export PEERS_LIST="sup-1=${REDIS_SUPERVISOR_1_NAME}:6001,sup-2=${REDIS_SUPERVISOR_2_NAME}:6002,sup-3=${REDIS_SUPERVISOR_3_NAME}:6003,sup-4=${REDIS_SUPERVISOR_4_NAME}:6004,sup-5=${REDIS_SUPERVISOR_5_NAME}:6005,sup-6=${REDIS_SUPERVISOR_6_NAME}:6006"
    export RAFT_NODES_URLS="http://agenda-db-raft-node-1:8001,http://agenda-db-raft-node-2:8002,http://agenda-db-raft-node-3:8003,http://agenda-db-raft-node-4:8004,http://agenda-db-raft-node-5:8005,http://agenda-db-raft-node-6:8006"

    echo "Redis environment variables have been exported to the current shell session."
    echo "These variables will be available until you close this terminal session."
}

# Execute the function
export_redis_vars