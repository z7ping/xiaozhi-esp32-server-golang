# Local deployment variant

This Compose file can use an external MySQL server, local images, and an optional read-only Jellyfin MCP adapter. It does not contain host addresses or credentials.

1. Copy `.env.example` to `.env` and set the host address, database connection, and image names. Use an address reachable by the ESP32 for `BIND_IP` when devices connect over the LAN.
2. Put the runtime server config at `../../../secrets/main-config.yaml` relative to this directory. Put the Jellyfin MCP environment file at `../../../secrets/jellyfin-mcp.env`. Both files must stay outside this Git repository; protect them with local filesystem permissions.
3. Run `docker compose config` to check the resolved configuration, then `docker compose up -d`.

The bundled `.env.example` values are placeholders. In particular, the public image tags may not include changes built locally for this deployment.
