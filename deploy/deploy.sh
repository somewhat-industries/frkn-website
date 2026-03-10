#!/bin/bash
# Deploy to VPS via Docker Compose
# Usage: ./deploy.sh
set -e

SERVER="wvProjects"
REMOTE_DIR="/opt/frkn"

echo "=== Syncing files to $SERVER..."
ssh $SERVER "mkdir -p $REMOTE_DIR"
rsync -avz --exclude='.git' --exclude='backend/data.db' \
  /Users/weristvlad/Documents/programming/FRKN-Website/ \
  $SERVER:$REMOTE_DIR/

echo "=== Building & restarting containers..."
ssh $SERVER "cd $REMOTE_DIR && docker compose pull nginx certbot && docker compose up -d --build"

echo "=== Status:"
ssh $SERVER "cd $REMOTE_DIR && docker compose ps"

echo "=== Logs (last 20 lines):"
ssh $SERVER "cd $REMOTE_DIR && docker compose logs --tail=20 backend"

echo "=== Deploy complete!"
