#!/bin/bash
set -e

echo "Stopping all running containers..."
docker ps -q | xargs -r docker stop

echo "Removing all containers (including Harbor and others)..."
docker ps -aq | xargs -r docker rm -f

echo "Removing Harbor Docker images..."
# Force remove Harbor-related images
docker images --filter=reference='goharbor/*' --format "{{.ID}}" | xargs -r docker rmi -f

echo "Removing all Docker volumes..."
docker volume ls --format "{{.Name}}" | xargs -r docker volume rm -f

echo "Removing all Docker networks..."
docker network ls --format "{{.ID}}" | xargs -r docker network rm -f || echo "Some networks could not be removed, possibly in use."

echo "Removing Harbor installer files..."
rm -rf harbor harbor-offline-installer-*

echo "Cleanup complete!"

