# Install

## One-liner (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/laneyweb/linux-ip/main/install.sh | bash
```

Installs the latest GitHub release binary for your arch (`x86_64`/`arm64`)
to `/usr/local/bin/linux-ip` with checksum verification when available.

No root? The script uses `sudo` when `/usr/local/bin` isn't writable,
otherwise falls back to `~/.local/bin/linux-ip` (it prints a `PATH` hint
if needed). Non-interactive sudo (password prompt with no TTY) also falls
back instead of failing.

Env overrides:

```bash
VERSION=v0.1.0 PREFIX=$HOME/.local/bin bash install.sh
```

## Manual

1. Open <https://github.com/laneyweb/linux-ip/releases>.
2. Download `linux-ip_<ver>_linux_<arch>.tar.gz`.
3. `tar -xzf linux-ip_*.tar.gz && sudo install -m 0755 linux-ip /usr/local/bin/`.

## From source

Requires Go ≥ 1.23:

```bash
git clone https://github.com/laneyweb/linux-ip.git
cd linux-ip
go build -o linux-ip ./cmd/linux-ip
./linux-ip --help
```

## Optional helpers (for full detail)

- Ubuntu: `sudo apt install iproute2 wireless-tools net-tools`
- Fedora 44: `sudo dnf install iproute iw net-tools`
- Clipboard: `wl-copy` (Wayland) or `xclip`/`xsel` (X11).
