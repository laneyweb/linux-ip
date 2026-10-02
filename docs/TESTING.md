# Testing (Ubuntu + Fedora 44)

## Ubuntu 24.04 (tested)

```bash
go vet ./...
go build -o linux-ip ./cmd/linux-ip
./linux-ip --no-color
./linux-ip --no-color --advanced
./linux-ip --json | jq .
./linux-ip --copy ip4
./linux-ip --no-color --public-ip   # opt-in external request
```

Expected: primary IPv4, gateway, DNS `127.0.0.53` (systemd-resolved),
`ss` ports, `ufw` detect-only.

## Fedora 44

```bash
sudo dnf install -y golang iproute iw net-tools wl-clipboard
go vet ./...
go build -o linux-ip ./cmd/linux-ip
./linux-ip --no-color --advanced
```

Checklist:

- [ ] `ip route` default gateway parses (NetworkManager keyfile, no netplan).
- [ ] `resolvectl status` per-link DNS shows under DNS DETAIL.
- [ ] `firewall-cmd --state` detected as `firewalld` (not `ufw`).
- [ ] SELinux: binary runs unconfined without AVC denials (`ausearch -m avc -ts recent`).
- [ ] Wayland clipboard via `wl-copy`; X11 via `xclip`.
- [ ] JSON validates: `./linux-ip --json | python3 -m json.tool`.

## Container smoke test (Fedora from Ubuntu host)

```bash
podman run --rm -v $PWD:/work -w /work fedora:44 bash -c \
  "dnf install -y golang iproute && go build -o /tmp/linux-ip ./cmd/linux-ip && /tmp/linux-ip --no-color"
```
