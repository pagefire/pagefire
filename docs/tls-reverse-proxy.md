# TLS & Reverse Proxy Setup

PageFire does not terminate TLS itself. In production, place a reverse proxy in front of PageFire to handle HTTPS. This keeps TLS certificate management separate from the application and follows standard deployment practice.

**Why this matters:** PageFire sets `Secure=true` on session cookies, so browsers will only send them over HTTPS. Without TLS in production, login sessions will not persist.

## Caddy (simplest option)

Caddy provides automatic HTTPS via Let's Encrypt with zero configuration. Create a `Caddyfile`:

```
pagefire.example.com {
    reverse_proxy localhost:3000
}
```

Start Caddy:

```bash
caddy run
```

That's it. Caddy automatically obtains and renews a TLS certificate from Let's Encrypt. It also sets `X-Forwarded-Proto` and other proxy headers by default.

## Nginx

### Install certbot and obtain a certificate

```bash
sudo apt install nginx certbot python3-certbot-nginx   # Debian/Ubuntu
sudo certbot --nginx -d pagefire.example.com
```

### Nginx configuration

Save this as `/etc/nginx/sites-available/pagefire` and symlink it to `sites-enabled`:

```nginx
server {
    listen 80;
    server_name pagefire.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name pagefire.example.com;

    ssl_certificate     /etc/letsencrypt/live/pagefire.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/pagefire.example.com/privkey.pem;

    # TLS settings
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 1d;

    location / {
        proxy_pass http://127.0.0.1:3000;

        # Required: tell PageFire the original protocol so it generates
        # correct URLs (invite links, redirects, etc.)
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Host              $host;

        # WebSocket support (used by the dashboard for live updates)
        proxy_http_version 1.1;
        proxy_set_header Upgrade    $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Sensible timeouts
        proxy_read_timeout 90s;
        proxy_send_timeout 90s;
    }
}
```

Enable and reload:

```bash
sudo ln -s /etc/nginx/sites-available/pagefire /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

### Auto-renewal

Certbot installs a systemd timer (or cron job) automatically. Verify with:

```bash
sudo certbot renew --dry-run
```

## Important Notes

**X-Forwarded-Proto header:** Make sure your reverse proxy sends `X-Forwarded-Proto: https`. PageFire uses this to generate correct absolute URLs in invite emails and API responses.

**Secure cookies require HTTPS:** Session cookies are set with `Secure=true`. On plain HTTP (except localhost), browsers will refuse to store them, and login will not work. Always terminate TLS in production.

**Changing the listen port:** PageFire defaults to port 3000. To change it, set the `PAGEFIRE_PORT` environment variable:

```bash
PAGEFIRE_PORT=8080 pagefire serve
```

Update `proxy_pass` (or Caddy's `reverse_proxy`) to match.

**localhost development:** Browsers treat `localhost` as a secure context, so `Secure` cookies work without TLS during local development. No reverse proxy is needed for `http://localhost:3000`.
