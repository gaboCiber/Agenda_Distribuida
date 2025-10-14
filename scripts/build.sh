#!/bin/bash

# Script para construir imágenes de Docker de los servicios
# Uso: 
#   Construir todas las imágenes: ./scripts/build.sh
#   Construir solo un servicio: ./scripts/build.sh group_service
#   Especificar tag: ./scripts/build.sh --tag v1.0 group_service
#   Construir múltiples servicios: ./scripts/build.sh group_service api_gateway

# Configuración
TAG="latest"
SERVICES_TO_BUILD=()
BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Todos los servicios disponibles
ALL_SERVICES=(
    "api_gateway"
    "users_service"
    "events_service"
    "group_service"
    "notifications_service"
    "streamlit_app"
)

# Procesar argumentos
while [[ $# -gt 0 ]]; do
    case $1 in
        --tag)
            TAG="$2"
            shift 2
            ;;
        --help|-h)
            echo "Uso: $0 [OPCIONES] [SERVICIOS...]"
            echo "Opciones:"
            echo "  --tag TAG      Especifica el tag para las imágenes (por defecto: latest)"
            echo "  --help, -h     Muestra esta ayuda"
            echo ""
            echo "Ejemplos:"
            echo "  $0                         # Construye todos los servicios"
            echo "  $0 group_service           # Construye solo el servicio de grupos"
            echo "  $0 --tag v1.0 group_service  # Construye con un tag específico"
            echo "  $0 group_service api_gateway # Construye múltiples servicios"
            exit 0
            ;;
        *)
            # Verificar si el argumento es un servicio válido
            if [[ " ${ALL_SERVICES[@]} " =~ " $1 " ]]; then
                SERVICES_TO_BUILD+=("$1")
            else
                echo "❌ Servicio desconocido: $1"
                echo "   Servicios disponibles: ${ALL_SERVICES[@]}"
                exit 1
            fi
            shift
            ;;
    esac
done

# Si no se especificaron servicios, construir todos
if [ ${#SERVICES_TO_BUILD[@]} -eq 0 ]; then
    echo "⚠️  No se especificaron servicios, construyendo todos por defecto"
    SERVICES_TO_BUILD=("${ALL_SERVICES[@]}")
fi

# Construir cada imagen
echo "🏗️  Construyendo imágenes con etiqueta: $TAG"
for service in "${SERVICES_TO_BUILD[@]}"; do
    service_name="${service%_service}"  # Eliminar _service si existe
    image_name="agenda-${service_name}"
    
    echo -e "\n🔨 Construyendo $image_name:$TAG desde $service..."
    docker build -t "$image_name:$TAG" -f "$BASE_DIR/services/$service/Dockerfile" "$BASE_DIR/services/$service/"
    
    if [ $? -eq 0 ]; then
        echo "✅ $image_name:$TAG construida correctamente"
    else
        echo "❌ Error al construir $image_name:$TAG"
        exit 1
    fi
done

echo -e "\n✨ Construcción completada exitosamente!"