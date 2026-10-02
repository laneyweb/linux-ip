// Package netinfo — top-level snapshot orchestration.
package netinfo

import "strings"

// Options controls opt-in / expensive collection.
type Options struct {
	// PublicIP enables the external lookup (default false for privacy).
	PublicIP bool
	// IncludeIPv6 keeps IPv6 addresses and v6 routes (default false).
	IncludeIPv6 bool
}

// filterIPv4Only drops IPv6 entries (used when --ip6 is not passed).
func filterIPv4Only(list []string) []string {
	out := list[:0]
	for _, entry := range list {
		keep := []string{}
		for _, ip := range strings.Fields(entry) {
			if !strings.Contains(ip, ":") {
				keep = append(keep, ip)
			}
		}
		if len(keep) > 0 {
			out = append(out, strings.Join(keep, " "))
		}
	}
	return out
}
// Collect gathers the full snapshot. It never returns an error: individual
// collectors degrade to empty/unknown so the UI always renders something.
func Collect(opts Options) Snapshot {
	s := Snapshot{}
	s.Hostname = hostname()
	s.Interfaces = collectInterfaces()
	s.Routes4, s.Routes6, s.Gateway4, s.Gateway6 = collectRoutes()
	s.DNS = collectDNS()
	s.WiFi = collectWiFi(s.Interfaces)
	s.Tailscale = collectTailscale()
	s.Firewall = collectFirewall()
	s.Listening = collectListening()
	s.CollectedWith = []string{"stdlib-net", "sysfs", "iproute2"}
	if ip, iface := primaryPick(s.Interfaces, s.Gateway4); ip != "" {
		s.PrimaryIPv4 = ip
		s.PrimaryIface = iface
	}
	// IPv6 is opt-in (--ip6): strip addrs/routes so text and JSON stay v4.
	// This also drops IPv6 DNS servers from the effective list.
	if !opts.IncludeIPv6 {
		for i := range s.Interfaces {
			s.Interfaces[i].IPv6 = nil
		}
		s.Routes6 = nil
		s.Gateway6 = ""
		s.DNS.Effective = filterIPv4Only(s.DNS.Effective)
		s.DNS.Resolved = filterIPv4Only(s.DNS.Resolved)
		if strings.Contains(s.DNS.Current, ":") {
			s.DNS.Current = ""
		}
	}
	// Privacy: public IP lookup only when explicitly requested.
	if opts.PublicIP {
		if ip, geo := fetchPublicIP(); ip != "" {
			s.PublicIP = ip
			s.PublicGeo = geo
			s.CollectedWith = append(s.CollectedWith, "api.ipify.org")
		}
	}
	return s
}
