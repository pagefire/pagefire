# Self-Hosting PageFire with Docker

## Quick Start

```bash
git clone https://github.com/pagefire/pagefire.git
cd pagefire
docker compose up -d
```

Open **http://localhost:3000**. On first launch you will see a setup wizard to create your admin account.

To use the pre-built image instead of building locally, edit `docker-compose.yml` and replace the `build: .` line with:

```yaml
image: ghcr.io/pagefire/pagefire:latest
```

## Configuration

All configuration is done through environment variables with the `PAGEFIRE_` prefix. Uncomment and set them in `docker-compose.yml` under the `environment` key.

### Core

| Variable | Default | Description |
|---|---|---|
| `PAGEFIRE_PORT` | `3000` | HTTP listen port |
| `PAGEFIRE_DATA_DIR` | `/data` | Directory for SQLite database and data files |
| `PAGEFIRE_DATABASE_DRIVER` | `sqlite` | `sqlite` or `postgres` |
| `PAGEFIRE_DATABASE_URL` | `<data_dir>/pagefire.db` | Database path (SQLite) or connection string (Postgres) |
| `PAGEFIRE_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### Engine

| Variable | Default | Description |
|---|---|---|
| `PAGEFIRE_ENGINE_INTERVAL_SECONDS` | `5` | How often the engine processes pending alerts and escalations |

### SMTP (email notifications)

| Variable | Default | Description |
|---|---|---|
| `PAGEFIRE_SMTP_HOST` | | SMTP server hostname |
| `PAGEFIRE_SMTP_PORT` | `587` | SMTP server port |
| `PAGEFIRE_SMTP_FROM` | | Sender email address |
| `PAGEFIRE_SMTP_USERNAME` | | SMTP auth username |
| `PAGEFIRE_SMTP_PASSWORD` | | SMTP auth password |

### Slack

| Variable | Default | Description |
|---|---|---|
| `PAGEFIRE_SLACK_BOT_TOKEN` | | Slack bot token for DM notifications |

### Twilio (SMS/voice)

| Variable | Default | Description |
|---|---|---|
| `PAGEFIRE_TWILIO_ACCOUNT_SID` | | Twilio account SID |
| `PAGEFIRE_TWILIO_AUTH_TOKEN` | | Twilio auth token |
| `PAGEFIRE_TWILIO_FROM_NUMBER` | | Twilio phone number (e.g. `+15551234567`) |

### Security

| Variable | Default | Description |
|---|---|---|
| `PAGEFIRE_ALLOW_PRIVATE_WEBHOOKS` | `false` | Allow webhook contact methods to target private/localhost IPs |

## Data Persistence

PageFire stores its SQLite database in the `/data` directory inside the container. The `docker-compose.yml` maps this to a named Docker volume (`pagefire-data`), so your data survives container restarts and recreations.

To use a bind mount instead (useful for backups):

```yaml
volumes:
  - ./data:/data
```

Make sure the host directory exists and is writable.

### Backups

With SQLite, you can back up the database by copying the file while PageFire is running (SQLite supports concurrent reads):

```bash
docker compose exec pagefire cp /data/pagefire.db /data/pagefire-backup.db
docker compose cp pagefire:/data/pagefire-backup.db ./pagefire-backup.db
```

## Creating an Admin User via CLI

If you need to create an admin user non-interactively (e.g. in CI or scripted setup):

```bash
docker compose exec pagefire pagefire admin create \
  --email admin@example.com \
  --name "Admin" \
  --password "your-secure-password"
```

## Updating

Pull the latest image and recreate the container. Your data is preserved in the volume.

```bash
docker compose pull        # if using a pre-built image
docker compose up -d --build  # if building from source
```

To pin a specific version:

```yaml
image: ghcr.io/pagefire/pagefire:v0.3.0
```

## Running Behind a Reverse Proxy

In production, run PageFire behind a reverse proxy (Caddy, nginx, Traefik) that terminates TLS. Example with Caddy added to the compose file:

```yaml
services:
  caddy:
    image: caddy:2
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy-data:/data
    depends_on:
      - pagefire

  pagefire:
    build: .
    # no need to expose port 3000 to the host
    expose:
      - "3000"
    # ... rest of config
```

With a `Caddyfile`:

```
pagefire.example.com {
    reverse_proxy pagefire:3000
}
```
