#!/bin/sh
set -e

mkdir -p /data/gogs/conf  # Ensure the directory exists
cp /app/app.ini /data/gogs/conf/app.ini

exec /app/gogs/docker/start.sh "$@"

