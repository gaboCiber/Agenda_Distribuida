#!/bin/bash

# Create images directory if it doesn't exist
mkdir -p images

# Get all agenda-* images and save them
docker images --format "{{.Repository}}:{{.Tag}}" | grep "^agenda-" | while read -r image; do
    # Replace / with _ in the image name for the filename
    filename=$(echo "$image" | tr '/' '_' | tr ':' '_').tar
    echo "Guardando $image en images/$filename"
    docker save -o "images/$filename" "$image"
done

echo "Todas las imágenes se han guardado en el directorio images/"