# StreamStatus Self-Hosting Deployment Guide

This guide provides step-by-step instructions for deploying the StreamStatus backend on your own custom Linux hardware or Virtual Private Server (VPS), completely avoiding Docker. It utilizes standard open-source tools: Systemd, PostgreSQL, and Nginx (or Caddy).

## Prerequisites

- A Linux server (Ubuntu/Debian or Alpine)
- Go 1.23+ installed (`sudo apt install golang` or from source)
- PostgreSQL installed (`sudo apt install postgresql`)
- A reverse proxy installed (Nginx or Caddy)
- A registered Twitch Developer Application (for Client ID and Secret)

## 1. Build the Binary

First, clone the repository and build the Go binary from source.

```bash
git clone https://github.com/infosecstreams-mirror/StreamStatus.git
cd StreamStatus/src
GOWORK=off go mod vendor
go build -mod=vendor -ldflags="-s -w" -o streamstatus ./...
```

Move the compiled binary to a system path (e.g., `/usr/local/bin/`).

```bash
sudo mv streamstatus /usr/local/bin/streamstatus
sudo chmod +x /usr/local/bin/streamstatus
```

## 2. PostgreSQL Setup

Log into your local Postgres instance and create the database and user.

```bash
sudo -u postgres psql
```

Execute the following SQL commands:

```sql
CREATE DATABASE streamstatus;
CREATE USER streamuser WITH ENCRYPTED PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE streamstatus TO streamuser;
\c streamstatus
GRANT ALL ON SCHEMA public TO streamuser;
```

The application will automatically create the necessary `streamers` table on startup.

## 3. Systemd Service & Environment Variables

We use Systemd to manage the lifecycle of the StreamStatus daemon.

Create an environment variables file to store your secrets securely:

```bash
sudo nano /etc/streamstatus.env
```

Add your configuration:

```env
# /etc/streamstatus.env
DATABASE_URL=postgres://streamuser:your_secure_password@localhost:5432/streamstatus?sslmode=disable
TW_CLIENT_ID=your_twitch_client_id
TW_CLIENT_SECRET=your_twitch_client_secret
SS_SECRETKEY=generate_a_random_secret_string
SS_ADMIN_TOKEN=your_admin_token
SS_CALLBACK_URL=https://your-domain.com/api/webhook
```

Restrict permissions on the secrets file:
```bash
sudo chmod 600 /etc/streamstatus.env
```

Next, create the Systemd unit file:

```bash
sudo nano /etc/systemd/system/streamstatus.service
```

```ini
[Unit]
Description=StreamStatus Backend API
After=network.target postgresql.service

[Service]
Type=simple
User=www-data
EnvironmentFile=/etc/streamstatus.env
ExecStart=/usr/local/bin/streamstatus
Restart=on-failure
RestartSec=5
# Principle of Least Privilege
ProtectSystem=full
ProtectHome=true
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable streamstatus
sudo systemctl start streamstatus
```

## 4. Reverse Proxy Setup (Nginx)

The StreamStatus API listens locally on port `8080`. You need a reverse proxy to expose it to the internet securely over HTTPS.

Create an Nginx configuration file:

```bash
sudo nano /etc/nginx/sites-available/streamstatus
```

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name your-domain.com;

    # Redirect HTTP to HTTPS (Make sure you configure Certbot/Let's Encrypt later)

    location /api/ {
        proxy_pass http://127.0.0.1:8080/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Enable the site and reload Nginx:
```bash
sudo ln -s /etc/nginx/sites-available/streamstatus /etc/nginx/sites-enabled/
sudo systemctl reload nginx
```

## 5. Verify Deployment

To ensure everything is working correctly:

1. Check the application logs: `sudo journalctl -u streamstatus -f`
2. Make a request to the API from the outside (since internal requests might not simulate your reverse proxy or firewall setup properly):
   ```bash
   curl -I https://your-domain.com/api/status
   ```
3. If you see an `HTTP/1.1 200 OK`, your deployment is successful!
