#!/bin/bash

# Check if images directory exists
if [ ! -d "images" ]; then
    echo "Error: 'images' directory not found."
    exit 1
fi

# Find and load all .tar files in the images directory
for image_file in images/*.tar; do
    if [ -f "$image_file" ]; then
        echo "Loading image from $image_file..."
        docker load -i "$image_file"
    else
        echo "No .tar files found in the images directory."
        exit 1
    fi
done

echo "All images have been loaded successfully."