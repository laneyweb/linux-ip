// Package ui — advanced sysadmin detail view (`linux-ip --advanced`).
package ui

import (
	"fmt"
	"strings"

	"github.com/laneyweb/linux-ip/internal/netinfo"
)

// RenderAdvanced prints basic view plus routes, DNS detail, ports, firewall.
func RenderAdvanced(s netinfo.Snapshot, t Theme) string {
	var b strings.Builder
	b.WriteString(RenderBasic(s, t))
	b.WriteString("\n" + t.Header.Render("── advanced ──") + "\n")

	b.WriteString(section(t, "ROUTES IPv4"))
	for _, r := range s.Routes4 {
		b.WriteString(fmt.Sprintf("  %-18s via %-15s dev %-10s %s\n",
			r.Destination, orDash(r.Gateway), orDash(r.Iface), t.Dim.Render(r.Proto)))
	}
	if len(s.Routes6) > 0 {
		b.WriteString(section(t, "ROUTES IPv6"))
		for _, r := range s.Routes6 {
			b.WriteString(fmt.Sprintf("  %-30s via %-30s dev %s\n",
				r.Destination, orDash(r.Gateway), orDash(r.Iface)))
		}
	} else if !t.ShowIPv6 {
		b.WriteString(t.Dim.Render("  (IPv6 hidden — use --ip6 to show v6 routes)\n"))
	}

	b.WriteString(section(t, "DNS DETAIL"))
	b.WriteString(fmt.Sprintf("  source: %s  symlink: %s  mode: %s\n",
		s.DNS.Source, orDash(s.DNS.SymlinkTo), s.DNS.Mode))
	// Effective first (what actually answers), raw stub file for transparency.
	b.WriteString(fmt.Sprintf("  effective: %s\n", strings.Join(s.DNS.DisplayServers(), ", ")))
	if s.DNS.Current != "" {
		b.WriteString(fmt.Sprintf("  current: %s (default-route link)\n", s.DNS.Current))
	}
	b.WriteString(fmt.Sprintf("  stub file: %s\n", strings.Join(s.DNS.Servers, ", ")))
	if len(s.DNS.Search) > 0 {
		b.WriteString(fmt.Sprintf("  search: %s\n", strings.Join(s.DNS.Search, ", ")))
	}
	for _, r := range s.DNS.Resolved {
		b.WriteString(fmt.Sprintf("  resolvectl: %s\n", r))
	}

	b.WriteString(section(t, "ALL INTERFACES"))
	for _, ii := range s.Interfaces {
		// IPv6 column only when --ip6; otherwise omit to keep rows short.
		if t.ShowIPv6 {
			v6 := addrsToString(ii.IPv6)
			b.WriteString(fmt.Sprintf("  %s %-10s mtu=%d mac=%s driver=%s duplex=%s ipv6=[%s]\n",
				t.dot(ii.IsUp), ii.Name, ii.MTU, orDash(ii.MAC),
				orDash(ii.Driver), orDash(ii.Duplex), orDash(v6)))
		} else {
			b.WriteString(fmt.Sprintf("  %s %-10s mtu=%d mac=%s driver=%s duplex=%s\n",
				t.dot(ii.IsUp), ii.Name, ii.MTU, orDash(ii.MAC),
				orDash(ii.Driver), orDash(ii.Duplex)))
		}
	}

	if s.Tailscale.Active || s.Tailscale.BackendState != "" {
		b.WriteString(section(t, "TAILSCALE"))
		b.WriteString(fmt.Sprintf("  state=%s self=%s host=%s dns=%s peers=%d\n",
			orDash(s.Tailscale.BackendState), orDash(s.Tailscale.SelfIP),
			orDash(s.Tailscale.Hostname), orDash(s.Tailscale.DNSName),
			s.Tailscale.PeerCount))
	}

	if len(s.WiFi) > 0 {
		b.WriteString(section(t, "WIFI"))
		for _, w := range s.WiFi {
			b.WriteString(fmt.Sprintf("  %s ssid=%s bssid=%s freq=%s signal=%s rate=%s\n",
				w.Iface, orDash(w.SSID), orDash(w.BSSID),
				orDash(w.Freq), orDash(w.Signal), orDash(w.Bitrate)))
		}
	}

	b.WriteString(section(t, "LISTENING PORTS"))
	if len(s.Listening) == 0 {
		b.WriteString("  (none detected — is `ss` installed?)\n")
	}
	for _, p := range s.Listening {
		if p.Process != "" {
			b.WriteString(fmt.Sprintf("  %-5s %-22s (%s)\n", p.Proto, p.Address, p.Process))
		} else {
			b.WriteString(fmt.Sprintf("  %-5s %s\n", p.Proto, p.Address))
		}
	}

	b.WriteString(section(t, "FIREWALL"))
	b.WriteString(fmt.Sprintf("  backend=%s active=%v — %s\n",
		s.Firewall.Backend, s.Firewall.Active, s.Firewall.Summary))
	return b.String()
}

func section(t Theme, name string) string {
	return "\n" + t.Header.Render(name) + "\n"
}
