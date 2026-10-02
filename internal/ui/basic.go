// Package ui — basic dashboard view (default `linux-ip` output).
package ui

import (
	"fmt"
	"strings"

	"github.com/laneyweb/linux-ip/internal/netinfo"
)

// RenderBasic shows one-screen summary: host, primary IP, gateway, DNS, WiFi.
func RenderBasic(s netinfo.Snapshot, t Theme) string {
	var b strings.Builder
	b.WriteString(t.Title.Render(fmt.Sprintf("linux-ip — basic ● %s", s.Hostname)) + "\n")

	primary := s.PrimaryIPv4
	if primary == "" {
		primary = "no global IPv4"
	}
	line := fmt.Sprintf("%s %s  %s %s  %s %s",
		t.Label.Render("PRIMARY"), t.Value.Render(primary),
		t.Label.Render("IFACE"), t.Value.Render(orDash(s.PrimaryIface)),
		t.Label.Render("GATEWAY"), t.Value.Render(orDash(s.Gateway4)),
	)
	b.WriteString(t.Box.Render(line) + "\n")

	// Interface rows: skip loopback in basic view for brevity.
	for _, ii := range s.Interfaces {
		if ii.IsLoopback {
			continue
		}
		b.WriteString(renderIfaceLine(ii, t) + "\n")
	}

	// DNS + WiFi one-liners.
	dns := strings.Join(s.DNS.Servers, ", ")
	if dns == "" {
		dns = "unknown"
	}
	b.WriteString(fmt.Sprintf("%s %s   %s %s\n",
		t.Label.Render("DNS:"), t.Value.Render(dns),
		t.Dim.Render("("+s.DNS.Mode+")"), t.Dim.Render(copyHint()),
	))
	for _, w := range s.WiFi {
		if w.State == "connected" {
			b.WriteString(fmt.Sprintf("%s %s  %s %s\n",
				t.Label.Render("WIFI:"), t.Value.Render(w.SSID),
				t.Label.Render("signal"), t.Value.Render(orDash(w.Signal)),
			))
		}
	}
	if s.PublicIP != "" {
		b.WriteString(fmt.Sprintf("%s %s\n", t.Label.Render("PUBLIC:"), t.Value.Render(s.PublicIP)))
	} else {
		b.WriteString(t.Dim.Render("PUBLIC: hidden (use --public-ip to opt in)") + "\n")
	}
	b.WriteString(t.Dim.Render("tips: -a advanced · --json · --copy ip4|gateway|dns|public") + "\n")
	return b.String()
}

// renderIfaceLine formats one interface as a status row.
func renderIfaceLine(ii netinfo.InterfaceInfo, t Theme) string {
	v4 := addrsToString(ii.IPv4)
	if v4 == "" {
		v4 = "no IPv4"
	}
	extra := ii.Kind
	if ii.SpeedMbps > 0 {
		extra = fmt.Sprintf("%s %d Mb/s", extra, ii.SpeedMbps)
	}
	return fmt.Sprintf("%s %-10s %s %-18s %s %s",
		t.dot(ii.IsUp), t.Value.Render(ii.Name),
		t.Dim.Render(ii.OperState), t.Value.Render(v4),
		t.Dim.Render(ii.MAC), t.Dim.Render(extra),
	)
}

// addrsToString joins addresses as "192.168.1.5/24, ...".
func addrsToString(addrs []netinfo.Addr) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		parts = append(parts, fmt.Sprintf("%s/%d", a.IP, a.PrefixLen))
	}
	return strings.Join(parts, ", ")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func copyHint() string { return "[--copy <field>]" }
