# linux-ip

Modern Linux networking dashboard for the terminal. Shows all IP / networking info in one view, with a basic and an advanced (`--advanced`) switch.

Built as a single Go binary — no Python, no pip — so the same file works on Ubuntu and Fedora 44.

## Quick start

```bash
curl -fsSL https://raw.githubusercontent.com/laneyweb/linux-ip/main/install.sh | bash
linux-ip               # basic view
linux-ip --advanced    # sysadmin detail (routes, DNS, ports, firewall)
linux-ip --json        # scriptable output
```

## Views

**Basic** (default): hostname, primary IPv4/iface, gateway, per-interface
IPv4/MAC/state, DNS servers, WiFi SSID (if any). Public IP is hidden unless
you opt in.

**Advanced** (`-a`, `--advanced`): everything in basic plus IPv4/IPv6 routing
tables, DNS detail (`/etc/resolv.conf` + `resolvectl`), all interfaces with
MTU/driver/duplex/IPv6, WiFi detail, listening ports (`ss`), firewall
detect-only (`ufw`/`firewalld`/`nft`/`iptables`), VPN/Docker/virtual flags.

## Flags

| Flag | Description |
|------|-------------|
| `-a, --advanced` | sysadmin detail view |
| `--json` | JSON output (for scripts) |
| `--no-color` | plain text (also honors `NO_COLOR`) |
| `--public-ip` | **opt-in** public IP lookup via `https://api.ipify.org` |
| `--copy <field>` | copy to clipboard: `ip4\|gateway\|dns\|public\|mac` |
| `-v, --version` | print version |

Clipboard backends: `wl-copy` (Wayland) → `xclip`/`xsel` (X11) → OSC52
(SSH). Prints the value with a hint when no backend is available.

## Examples

```bash
linux-ip --no-color
linux-ip -a
linux-ip --json | jq .interfaces
linux-ip --copy ip4          # primary IPv4 to clipboard
linux-ip --public-ip         # include public IP (external request)
```

Example basic output:

```
linux-ip — basic ● myhost
PRIMARY 192.168.1.42  IFACE wlp2s0  GATEWAY 192.168.1.1
[up] wlp2s0  192.168.1.42/24  9c:6b:00:xx:xx:xx  wifi
DNS: 1.1.1.1, 8.8.8.8  (systemd-resolved)
WIFI: HomeNet  signal -52 dBm
PUBLIC: hidden (use --public-ip to opt in)
```

## Distro support

| Distro | Tested | Notes |
|--------|--------|-------|
| Ubuntu 24.04 | ✅ | `apt install iproute2 wireless-tools` for full detail |
| Fedora 44 | ⏳ | uses NetworkManager + firewalld paths, see `docs/TESTING.md` |

Portable sources only: Go stdlib + `/sys/class/net` + `iproute2`.
`nmcli`/`iw`/`ethtool`/`ss`/`resolvectl` are best-effort. Never requires root.

## Install options

- **Script (recommended):** `install.sh` pulls the latest GitHub release tarball for your arch into `/usr/local/bin`.
- **Manual:** download `linux-ip_*_x86_64.tar.gz` from Releases, extract, move to `PATH`.
- **From source:** `go build -o linux-ip ./cmd/linux-ip` (Go ≥ 1.23).

See `docs/INSTALL.md` and `docs/TESTING.md`. Man page: `man/linux-ip.1`.

## Privacy

No telemetry. LAN data never leaves the machine. The only network request
is the opt-in `--public-ip` lookup.

## Versioning

SemVer (`v0.1.0`, `v0.2.0`…). Tags + GitHub Releases + `CHANGELOG.md`.
