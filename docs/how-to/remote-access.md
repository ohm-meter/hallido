# How-to: Remote access (from outside your home)

Problem: You want the web app or API to be reachable outside your home
network — without exposing the agent unprotected to the internet.

## Ground rule

The agent speaks **HTTP only** and protects the API with a static token.
**Do not forward port `8080`** on your router. Use one of the following
options instead — each with **TLS** and an **authentication layer in front**, so
nobody gets as far as the token prompt without logging in.

| Option | Suited for | Effort |
|--------|------------|--------|
| **VPN** (WireGuard, Tailscale, etc.) | your own devices | low, safest choice |
| **Zero-trust tunnel** (e.g. Cloudflare Tunnel + Access) | browser access without a VPN app, your own domain | medium |
| **Reverse proxy** (Caddy, nginx, Traefik) with login (e.g. OAuth2 Proxy, Authelia) | an existing server with port 443 open | medium to high |

## VPN option

1. Set up the VPN on your router or a machine on the LAN.
2. Connect the VPN on your phone/laptop.
3. Open the agent as you would at home: `http://raspberrypi.local:8080` or the LAN IP.

No further steps on the agent are needed.

## Tunnel or reverse proxy option

1. Set up the tunnel client or proxy on the agent host (or a host on the LAN),
   targeting `http://127.0.0.1:8080` or `http://<LAN-IP>:8080`.
2. If it runs on the same host, bind the agent to localhost only — in `config.json`:

   ```json
   "listen": "127.0.0.1:8080"
   ```

   and restart the service (`sudo systemctl restart sonos-agent`).
3. Create a public hostname with TLS, e.g. `https://sonos.example.com`.
4. **Put authentication in front** (access policy, OAuth2 Proxy, etc.), allowing
   only your own accounts/email addresses.
5. Optionally block `GET /api/v1/health` externally in the proxy (it returns the
   names of the reachable speakers without a token).

### Things to watch

- **Caching:** The agent serves the page and manifest with `no-cache` and
  versioned assets (`?v=…`). Proxies/CDNs must not cache the page for longer,
  or the app will show stale versions after an update.
- **Installing the web app:** Works only over HTTPS (or `localhost`) —
  see [Install as an app](install-as-app.md). With authentication in front, the
  manifest must be loaded with login cookies; the app sets
  `crossorigin="use-credentials"` for this.
- **Homey/automations:** They run on the LAN and talk to the agent directly, not
  through the external hostname.
- **Rate limit:** Behind a proxy the agent sees only the proxy's address; all
  external clients then share one limit.

## Verify

```bash
curl -I https://sonos.example.com/            # without login: redirect to sign-in, not the app
curl http://127.0.0.1:8080/api/v1/health      # on the host: agent is running
```

See also: [SECURITY.md](../../SECURITY.md), [REST API](../reference/api.md).
