#!/bin/bash

set -e

echo "📁 Creating Gogs deployment directory..."
mkdir -p ~/gogs-docker
cd ~/gogs-docker

echo "📝 Writing docker-compose.yml..."
cat <<EOF > docker-compose.yml
version: '3'

services:
  gogs:
    image: gogs/gogs:latest
    container_name: gogs
    ports:
      - "3000:3000"
      - "10022:22"
    volumes:
      - ./gogs-data:/data
    restart: unless-stopped
EOF

echo "🚀 Starting Gogs with Docker Compose..."
docker-compose up -d

echo "✅ Gogs deployed successfully!"
echo "🌐 Access Gogs at: http://$(curl -s ifconfig.me):3000"
echo "📦 Gogs data stored in: ~/gogs-docker/gogs-data"

