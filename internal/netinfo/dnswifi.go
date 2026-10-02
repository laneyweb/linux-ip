// Package netinfo — DNS, WiFi, host identity, public IP (opt-in).
package netinfo

import (
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// collectDNS resolves the *effective* uplink DNS, not just the stub.
//
// Why: on systemd-resolved systems /etc/resolv.conf contains only
// 127.0.0.53 (the local stub). The servers actually answering queries are:
//   1. /run/systemd/resolve/resolv.conf (all known uplink servers), and
//   2. `resolvectl status` per-link "Current DNS Server" entries.
// VPNs (NordVPN/nordlynx with DNS Domain ~.) and Tailscale (100.100.100.100)
// each add their own link, so we track the default-route link's Current
// server separately (usually the LAN DNS, e.g. 192.168.1.188).
func collectDNS() DNSInfo {
	d := DNSInfo{Source: "/etc/resolv.conf"}
	if target, err := os.Readlink("/etc/resolv.conf"); err == nil {
		d.SymlinkTo = target
		if strings.Contains(target, "systemd") {
			d.Mode = "systemd-resolved"
		}
	}
	if b, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(strings.TrimSpace(line))
			if len(f) < 2 || strings.HasPrefix(f[0], "#") {
				continue
			}
			switch f[0] {
			case "nameserver":
				d.Servers = append(d.Servers, f[1])
			case "search":
				d.Search = append(d.Search, f[1:]...)
			}
		}
	}
	if d.Mode == "" {
		if len(d.Servers) == 1 && isStub(d.Servers[0]) {
			d.Mode = "stub"
		} else {
			d.Mode = "static"
		}
	}
	// Source 1: uplink file with the real servers (stub never listed here).
	if b, err := os.ReadFile("/run/systemd/resolve/resolv.conf"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(strings.TrimSpace(line))
			if len(f) == 2 && f[0] == "nameserver" && !isStub(f[1]) {
				d.Effective = appendIfMissing(d.Effective, f[1])
			}
		}
	}
	// Source 2: resolvectl per-link servers + Current DNS of default route.
	if out, err := exec.Command("resolvectl", "status").Output(); err == nil {
		current, defaultCurrent := parseResolvectl(string(out))
		d.Resolved = current
		d.Current = defaultCurrent
		// Prefer the default-route link's Current server first (LAN DNS),
		// then remaining uplink servers. Stub entries are dropped.
		ordered := []string{}
		if d.Current != "" && !isStub(d.Current) {
			ordered = append(ordered, d.Current)
		}
		for _, entry := range current {
			for _, ip := range strings.Fields(entry) {
				if !isStub(ip) {
					ordered = appendIfMissing(ordered, ip)
				}
			}
		}
		// Merge with the uplink file (keeps file order for the rest).
		for _, ip := range ordered {
			d.Effective = appendIfMissing(d.Effective, ip)
		}
		// Move Current to front so display order is LAN-first.
		if d.Current != "" {
			front := []string{d.Current}
			for _, ip := range d.Effective {
				if ip != d.Current {
					front = append(front, ip)
				}
			}
			d.Effective = front
		}
	}
	return d
}

// isStub reports loopback stub resolver addresses (not real upstream DNS).
func isStub(ip string) bool {
	return ip == "127.0.0.53" || ip == "127.0.0.1" || ip == "::1"
}

func appendIfMissing(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// parseResolvectl extracts per-link "DNS Servers:" entries and the Current
// DNS Server of the link marked +DefaultRoute (the LAN link).
func parseResolvectl(s string) (servers []string, defaultCurrent string) {
	var curCurrent string
	var curDefault bool
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.Contains(line, "+DefaultRoute"):
			curDefault = true
		case strings.HasPrefix(line, "Link "):
			// New link block: commit previous tracking state.
			curCurrent = ""
			curDefault = strings.Contains(line, "+DefaultRoute")
		case strings.HasPrefix(line, "Current DNS Server:"):
			curCurrent = strings.TrimSpace(strings.TrimPrefix(line, "Current DNS Server:"))
			if curDefault && defaultCurrent == "" && curCurrent != "" {
				defaultCurrent = curCurrent
			}
		case strings.HasPrefix(line, "DNS Servers:"):
			servers = append(servers, strings.TrimSpace(strings.TrimPrefix(line, "DNS Servers:")))
		}
	}
	return servers, defaultCurrent
}

// collectWiFi queries `iw` then `nmcli` (both optional) per wireless iface.
func collectWiFi(ifaces []InterfaceInfo) []WiFiInfo {
	var out []WiFiInfo
	for _, ii := range ifaces {
		if !ii.IsWireless {
			continue
		}
		w := WiFiInfo{Iface: ii.Name, State: "disconnected"}
		// Preferred: `iw dev <iface> link` — no root needed for link status.
		if o, err := exec.Command("iw", "dev", ii.Name, "link").Output(); err == nil {
			s := string(o)
			if strings.Contains(s, "Not connected") {
				w.State = "disconnected"
			} else {
				w.State = "connected"
				for _, line := range strings.Split(s, "\n") {
					line = strings.TrimSpace(line)
					switch {
					case strings.HasPrefix(line, "SSID:"):
						w.SSID = strings.TrimSpace(strings.TrimPrefix(line, "SSID:"))
					case strings.HasPrefix(line, "freq:"):
						w.Freq = strings.TrimSpace(strings.TrimPrefix(line, "freq:"))
					case strings.HasPrefix(line, "signal:"):
						w.Signal = strings.TrimSpace(strings.TrimPrefix(line, "signal:"))
					case strings.HasPrefix(line, "rx bitrate:"):
						w.Bitrate = strings.TrimSpace(strings.TrimPrefix(line, "rx bitrate:"))
					}
				}
			}
			out = append(out, w)
			continue
		}
		// Fallback: NetworkManager CLI (present on both test distros).
		if o, err := exec.Command("nmcli", "-t", "-f", "IN-USE,SSID,SIGNAL", "dev", "wifi").Output(); err == nil {
			for _, line := range strings.Split(string(o), "\n") {
				f := strings.Split(line, ":")
				if len(f) >= 2 && strings.TrimSpace(f[0]) == "*" {
					w.State = "connected"
					w.SSID = strings.TrimSpace(f[1])
					if len(f) >= 3 {
						w.Signal = strings.TrimSpace(f[2]) + "%"
					}
				}
			}
		}
		out = append(out, w)
	}
	return out
}

// hostname returns the machine hostname, never empty.
func hostname() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "unknown"
}

// primaryPick chooses the interface owning the default route when possible,
// falling back to the first non-loopback interface with a global IPv4.
// This avoids picking docker0 (172.17.0.1) on hosts with many bridges.
func primaryPick(ifaces []InterfaceInfo, gw4 string) (ip, iface string) {
	// Pass 1: interface(s) on the default route path is resolved by the
	// caller via Gateway4; here we prefer non-virtual, non-container IPs.
	candidates := func(skipVirtual bool) (string, string) {
		for _, ii := range ifaces {
			if ii.IsLoopback || len(ii.IPv4) == 0 {
				continue
			}
			if skipVirtual && ii.IsVirtual {
				continue
			}
			for _, a := range ii.IPv4 {
				if a.Scope == "global" {
					return a.IP, ii.Name
				}
			}
		}
		return "", ""
	}
	if ip, iface := candidates(true); ip != "" {
		return ip, iface
	}
	return candidates(false)
}

// fetchPublicIP is OPT-IN only (--public-ip). Short timeout, no retries.
func fetchPublicIP() (ip, geo string) {
	client := &http.Client{Timeout: 4 * time.Second}
	// ipify returns plain-text IP; geo lookup kept minimal to avoid trackers.
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", ""
	}
	ip = strings.TrimSpace(string(b))
	if net.ParseIP(ip) == nil {
		return "", ""
	}
	_ = filepath.Base // keep filepath import if unused in future edits
	return ip, ""
}
