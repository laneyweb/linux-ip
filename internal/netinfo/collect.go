// Package netinfo — top-level snapshot orchestration.
package netinfo

// Options controls opt-in / expensive collection.
type Options struct {
	// PublicIP enables the external lookup (default false for privacy).
	PublicIP bool
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
	s.Firewall = collectFirewall()
	s.Listening = collectListening()
	s.CollectedWith = []string{"stdlib-net", "sysfs", "iproute2"}
	if ip, iface := primaryPick(s.Interfaces, s.Gateway4); ip != "" {
		s.PrimaryIPv4 = ip
		s.PrimaryIface = iface
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
