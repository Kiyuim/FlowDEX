# Nginx HTTPS Reverse Proxy Setup for web3ite.cab

## Overview

Configure nginx to serve `web3ite.cab` over HTTPS using the provided SSL certificates (`web3ite.cab_cert_chain.pem` and `web3ite.cab.key`), proxying all requests to `localhost:3001`.

## Files and Locations

### SSL Certificates

- **Source**: `/home/ubuntu/dex_full/web3ite.cab_cert_chain.pem` and `/home/ubuntu/dex_full/web3ite.cab.key`
- **Destination**: `/etc/ssl/certs/web3ite.cab_cert_chain.pem` and `/etc/ssl/private/web3ite.cab.key`
- **Permissions**: Cert chain: `644`, Private key: `600` (root:root)

### Nginx Configuration

- **File**: `/etc/nginx/sites-available/web3ite.cab`
- **Symlink**: `/etc/nginx/sites-enabled/web3ite.cab` (to enable the site)

## Implementation Steps

### 1. Copy SSL Certificates

- Copy certificate chain to `/etc/ssl/certs/web3ite.cab_cert_chain.pem`
- Copy private key to `/etc/ssl/private/web3ite.cab.key`
- Set appropriate permissions (cert: 644, key: 600)
- Ensure private key is owned by root

### 2. Create Nginx Configuration

Create `/etc/nginx/sites-available/web3ite.cab` with:

- Server block listening on port 443 (HTTPS)
- SSL configuration using the certificate files
- Reverse proxy to `http://localhost:3001`
- Proper headers for proxying (Host, X-Real-IP, X-Forwarded-For, X-Forwarded-Proto)
- Security headers (SSL protocols, ciphers)
- WebSocket support (if needed for the app)

### 3. Enable Site

- Create symlink from `sites-available` to `sites-enabled`
- Test nginx configuration with `nginx -t`
- Reload nginx service

### 4. Firewall Configuration (if applicable)

- Ensure port 443 is open in firewall rules

## Nginx Configuration Structure

```nginx
server {
    listen 443 ssl http2;
    server_name web3ite.cab;

    ssl_certificate /etc/ssl/certs/web3ite.cab_cert_chain.pem;
    ssl_certificate_key /etc/ssl/private/web3ite.cab.key;
    
    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # Proxy settings
    location / {
        proxy_pass http://localhost:3001;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## Verification Steps

1. Verify certificate files are in place with correct permissions
2. Test nginx configuration: `sudo nginx -t`
3. Check nginx status: `sudo systemctl status nginx`
4. Test HTTPS connection: `curl -k https://web3ite.cab` or browse to `https://web3ite.cab`
5. Verify proxy is working by checking response headers

## Notes

- The configuration assumes nginx is already installed
- May require sudo/root privileges for certificate placement and nginx configuration
- If the app on port 3001 requires WebSocket connections, the configuration includes WebSocket support
- Consider adding rate limiting, logging, and other security measures as needed