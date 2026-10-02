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

// collectDNS parses /etc/resolv.conf and enriches with resolvectl (best-effort).
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
		if len(d.Servers) == 1 && strings.HasPrefix(d.Servers[0], "127.0.0") {
			d.Mode = "stub"
		} else {
			d.Mode = "static"
		}
	}
	// resolvectl gives per-link servers on systemd distros (both Ubuntu/Fedora).
	if out, err := exec.Command("resolvectl", "status").Output(); err == nil {
		d.Resolved = parseResolved(string(out))
	}
	return d
}

// parseResolved extracts "DNS Servers:" lines from resolvectl output.
func parseResolved(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "DNS Servers:") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "DNS Servers:")))
		}
	}
	return out
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
