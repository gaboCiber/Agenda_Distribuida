#!/bin/bash

# Exit on error
set -e

# Get the absolute path of the project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Available services
SERVICES=("db" "user" "group" "api")

# Function to show usage
show_usage() {
    echo "Usage: $0 [all|db|user|group] [service2] [service3]..."
    echo "  all      Build all services (default if no arguments provided)"
    echo "  db       Build only the database service"
    echo "  user     Build only the user service"
    echo "  group    Build only the group service"
    echo ""
    echo "Examples:"
    echo "  $0 all           # Build all services"
    echo "  $0 db user       # Build only db and user services"
    echo "  $0 group         # Build only group service"
    exit 1
}

# Function to build a service
build_service() {
    local service_name
    local image_name

    if [ "${1}" == "api" ]; then
        service_name="api_gateway_service"
        image_name="agenda-api-gateway"
    else
        service_name="${1}_service"
        image_name="agenda-${1}_event"
    fi

    local context_path="$PROJECT_ROOT/services/${service_name}"

    echo "\n=== Building ${service_name} service ==="
    if [ -f "${context_path}/Dockerfile" ]; then
        docker build -t ${image_name} -f "${context_path}/Dockerfile" "${context_path}"
        echo "✅ Successfully built ${image_name}"
        return 0
    else
        echo "❌ Dockerfile not found for ${service_name} service"
        return 1
    fi
}

# If no arguments, build all services
if [ $# -eq 0 ]; then
    SERVICES_TO_BUILD=("${SERVICES[@]}")
else
    # Check if 'all' is specified
    if [[ " $* " =~ \ all\  ]]; then
        SERVICES_TO_BUILD=("${SERVICES[@]}")
    else
        # Validate services
        for service in "$@"; do
            if [[ ! " ${SERVICES[@]} " =~ " ${service} " ]]; then
                echo "❌ Error: Unknown service '$service'"
                show_usage
            fi
        done
        SERVICES_TO_BUILD=("$@")
    fi
fi

echo "Starting build process..."

# Build selected services
for service in "${SERVICES_TO_BUILD[@]}"; do
    build_service "$service" || exit 1
done

# Special handling for Redis
echo "\n=== Redis will use the official image: redis:7-alpine ==="

echo "\n=== Build completed successfully! ==="
echo "You can now start the services using: ./scripts_new/start.sh [all|redis|db|user|group]"
