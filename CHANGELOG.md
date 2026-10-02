# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.2.0] - 2026-10-02
### Fixed
- DNS now shows effective uplink servers (e.g. 192.168.1.188 from the
  default-route link) instead of the 127.0.0.53 systemd-resolved stub.
  Stub file content kept visible under `--advanced` → DNS DETAIL.
### Added
- Tailscale awareness: `TAILSCALE:` line in basic view, full section in
  advanced (self 100.x IP, hostname, peer count), `--copy tailscale`.
- `--ip6` / `--ipv6` flag: IPv6 addresses, v6 routes, and v6 DNS are now
  hidden by default and only included when requested (text + JSON).
- DNS moved into the PRIMARY header box.

## [v0.1.0] - 2026-10-02
### Added
- Initial release: Go single binary `linux-ip`.
- Basic dashboard (hostname, primary IP, gateway, interfaces, DNS, WiFi).
- Advanced view (`--advanced`): routes v4/v6, DNS detail, all interfaces,
  WiFi detail, listening ports, firewall detect-only.
- Flags: `--json`, `--no-color` (+ `NO_COLOR`), `--public-ip` opt-in,
  `--copy ip4|gateway|dns|public|mac`, `--version`.
- Clipboard: wl-copy → xclip/xsel → OSC52 fallback.
- Portable collectors: stdlib + sysfs + iproute2; no root required.
- Docs: README, INSTALL, TESTING, man page.
- Packaging: `install.sh` (curl), GoReleaser, GitHub Actions CI.
- Tested on Ubuntu 24.04; Fedora 44 pending (see docs/TESTING.md).
