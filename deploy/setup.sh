#!/bin/bash
# Run on the VPS as root: bash setup.sh
set -e

echo "=== FRKN Backend Setup ==="

# 1. Create user
useradd -r -s /bin/false frkn 2>/dev/null || true
mkdir -p /opt/frkn/static

# 2. Install dependencies
apt-get update -qq
apt-get install -y nginx certbot python3-certbot-nginx

# 3. Copy files
cp frkn-backend       /opt/frkn/frkn-backend
cp -r static/*        /opt/frkn/static/
cp frkn-backend.service /etc/systemd/system/
cp nginx.conf         /etc/nginx/sites-available/frknsw.com

# 4. Permissions
chown -R frkn:frkn /opt/frkn
chmod +x /opt/frkn/frkn-backend

# 5. Nginx
ln -sf /etc/nginx/sites-available/frknsw.com /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx

# 6. SSL (replace with actual email)
certbot --nginx -d frknsw.com -d www.frknsw.com --non-interactive --agree-tos -m admin@frknsw.com

# 7. Start service
systemctl daemon-reload
systemctl enable frkn-backend
systemctl start frkn-backend

echo "=== Done. Status:"
systemctl status frkn-backend --no-pager
