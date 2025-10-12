#!/bin/bash

# Script para construir las imágenes de Docker de todos los servicios
# Uso: ./scripts/build-images.sh [--tag TAG]

# Configuración
TAG="latest"

# Procesar argumentos
while [[ $# -gt 0 ]]; do
    case $1 in
        --tag)
            TAG="$2"
            shift 2
            ;;
        *)
            echo "Uso: $0 [--tag TAG]"
            exit 1
            ;;
    esac
done

# Directorio base del proyecto
BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Servicios a construir
SERVICES=(
    "api_gateway"
    "users_service"
    "events_service"
    "group_service"
    "notifications_service"
    "streamlit_app"
)

# Construir cada imagen
echo "Construyendo imágenes con etiqueta: $TAG"
for service in "${SERVICES[@]}"; do
    service_name="${service%_service}"  
    image_name="agenda-${service_name}"
    
    echo "\n--- Construyendo $image_name:$TAG ---"
    docker build -t "$image_name:$TAG" -f "$BASE_DIR/services/$service/Dockerfile" "$BASE_DIR/services/$service/"
    
    if [ $? -eq 0 ]; then
        echo "✅ $image_name:$TAG construida correctamente"
    else
        echo "❌ Error al construir $image_name:$TAG"
        exit 1
    fi
done

echo "\nTodas las imágenes han sido construidas exitosamente!"

# Mostrar las imágenes creadas
echo "\nImágenes creadas:"
docker images | grep agenda-
