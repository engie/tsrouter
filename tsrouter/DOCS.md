# Tailscale LAN Proxy

Each entry under `proxies` becomes its own node on your tailnet. TCP
connections to that node are forwarded to a device on the LAN that Home
Assistant can reach. The device needs no changes, and nothing else on the
LAN is exposed. This is the per-device alternative to a subnet router or
exit node.

## Example

```yaml
auth_key: tskey-auth-...        # optional, see below
tags:
  - tag:printer
proxies:
  - hostname: receipt
    target: 192.168.1.30
    ports: "9100,6101,80"
  - hostname: hp
    target: 192.168.1.136        # ports omitted: forward every TCP port
```

After this, `receipt:9100` and `hp:631` work from anywhere on your tailnet.
MagicDNS also gives the full `receipt.<tailnet>.ts.net` name.

### `ports`

- Empty (or `*`) forwards every TCP port to the same port on the target.
- `9100,631` forwards only those ports. Connections to any other port are refused.
- `8080:80` maps port 8080 on the tailnet to port 80 on the target.

### Authentication

Each node has to join the tailnet once. Its identity is then kept in
`/data/nodes/<hostname>`, so it survives restarts and upgrades. The options
are:

- **No `auth_key`**: the log prints a login URL for each new node. Open each
  one to approve it.
- **Auth key** (`tskey-auth-...`): use a reusable key, so one key covers
  every node.
- **OAuth client secret** (`tskey-client-...`): requires `tags`. The nodes
  are owned by the tags and their keys never expire.

If you use an auth key without tags, the nodes are owned by your user and
their keys expire. Disable key expiry for them in the admin console.

## Limits

- TCP only. UDP (SNMP printer status, mDNS) is not forwarded.
- No auto-discovery. Bonjour/AirPrint does not travel over Tailscale, so add
  printers by hostname. On macOS, add an IPP printer with address `hp`. On iOS,
  use an AirPrint configuration profile that points at the hostname.
- The LAN device sees every connection as coming from Home Assistant.
- Names like `printer.local` don't resolve inside the add-on. Use an IP
  address or a hostname from your router's DNS.

## Access control

Proxied devices usually have no authentication of their own, so restrict
who can reach them with your tailnet policy. For example:

```json
{"action": "accept", "src": ["autogroup:member"], "dst": ["tag:printer:9100,631,80,443"]}
```
