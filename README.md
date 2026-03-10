# FRKN Website — frknsw.com

Crowdsourced internet censorship map for Russia.

## Stack

- **Backend**: Go + SQLite (WAL mode), Docker
- **Frontend**: Leaflet.js, static HTML/CSS/JS
- **Server**: Docker Compose + Nginx + Certbot SSL

## First deploy on VPS

```bash
# 1. SSH to server
ssh wvProjects

# 2. Install Docker
curl -fsSL https://get.docker.com | sh
usermod -aG docker $USER

# 3. Clone / upload files (from local machine)
exit
./deploy/deploy.sh

# 4. Back on server — get SSL cert (HTTP must work first)
ssh wvProjects
cd /opt/frkn

# First: start with HTTP only (comment out ssl lines in nginx.conf temporarily)
docker compose up -d nginx backend

# Get cert
docker compose run --rm certbot certonly \
  --webroot -w /var/www/certbot \
  -d frknsw.com -d www.frknsw.com \
  --email admin@frknsw.com --agree-tos --no-eff-email

# Now enable HTTPS in nginx.conf, reload
docker compose exec nginx nginx -s reload
```

## Update deploy

```bash
cd /Users/weristvlad/Documents/programming/FRKN-Website
./deploy/deploy.sh
```

## API

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/report` | Submit a data point from iOS app |
| GET | `/api/map?zoom=N&bounds=lat1,lon1,lat2,lon2` | Clustered map data |
| GET | `/api/stats` | Total reports, last 24h, breakdown |

### POST /api/report payload

```json
{
  "lat": 55.74,
  "lon": 37.62,
  "diagnosis": "whitelist_active",
  "carrier": "МТС",
  "appVersion": "1.0",
  "networkType": "cellular"
}
```

## Server requirements

- 2 CPU cores, 2GB RAM — sufficient
- Backend uses ~20-50MB RAM
- SQLite handles millions of rows efficiently (WAL mode)
- Nginx + Certbot for SSL
