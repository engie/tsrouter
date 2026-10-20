# tsrouter

A Home Assistant add-on that gives each configured LAN device its own
Tailscale node and hostname, forwarding plain TCP to it. A printer at
`192.168.1.30` becomes `receipt` on your tailnet, without a subnet router or
exit node exposing the whole LAN.

Built on [tsnet](https://pkg.go.dev/tailscale.com/tsnet): one in-process
userspace Tailscale node per proxy, so it needs no TUN device, no extra
privileges and no host networking.

## Install

In Home Assistant, go to **Settings → Add-ons → Add-on Store → ⋮ →
Repositories** and add this repository's URL. Then install **Tailscale LAN
Proxy**. See [tsrouter/DOCS.md](tsrouter/DOCS.md) for configuration.

For local development, copy the `tsrouter/` directory into the Home
Assistant `/addons` share (Samba or SSH add-on). It then appears under
**Local add-ons**.

## Development

```sh
cd tsrouter
go test -race ./...
docker build -t tsrouter .
```
