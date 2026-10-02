// Package netinfo collects Linux networking state using portable sources.
//
// Portability strategy (Ubuntu + Fedora 44):
//   - Prefer Go stdlib (net.Interfaces) and sysfs (/sys/class/net).
//   - Fall back to `iproute2` (`ip route`) which exists on both distros.
//   - Treat NetworkManager / systemd / iw / ethtool output as best-effort.
//   - Never require root; degraded fields report "unknown" instead of failing.
package netinfo

import "net"

// Addr is a single IP address with its prefix length.
type Addr struct {
	IP        string `json:"ip"`
	PrefixLen int    `json:"prefixLen"`
	Scope     string `json:"scope,omitempty"` // global, link, host
}

// InterfaceInfo describes one network interface.
type InterfaceInfo struct {
	Name       string `json:"name"`
	MAC        string `json:"mac"`
	MTU        int    `json:"mtu"`
	OperState  string `json:"operState"` // up, down, dormant, unknown
	IsUp       bool   `json:"isUp"`
	IsLoopback bool   `json:"isLoopback"`
	IsWireless bool   `json:"isWireless"`
	IsVirtual  bool   `json:"isVirtual"` // docker, veth, bridge, VPN
	Kind       string `json:"kind"`      // ethernet, wifi, loopback, virtual, other
	IPv4       []Addr `json:"ipv4"`
	IPv6       []Addr `json:"ipv6"`
	SpeedMbps  int    `json:"speedMbps,omitempty"` // -1/0 = unknown
	Duplex     string `json:"duplex,omitempty"`    // full, half, unknown
	Driver     string `json:"driver,omitempty"`
}

// Route is a single routing table entry (v4 or v6).
type Route struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Iface       string `json:"iface,omitempty"`
	Proto       string `json:"proto,omitempty"`
	Metric      int    `json:"metric,omitempty"`
	Family      string `json:"family"` // ipv4 / ipv6
}

// DNSInfo aggregates resolver configuration.
type DNSInfo struct {
	Servers   []string `json:"servers"`   // from /etc/resolv.conf
	Search    []string `json:"search"`    // search domains
	Mode      string   `json:"mode"`      // stub, static, systemd-resolved, unknown
	Resolved  []string `json:"resolved"`  // per-link servers from resolvectl (best-effort)
	Source    string   `json:"source"`    // /etc/resolv.conf, etc.
	SymlinkTo string   `json:"symlinkTo"` // where /etc/resolv.conf points
}

// WiFiInfo is best-effort wireless status for one interface.
type WiFiInfo struct {
	Iface   string `json:"iface"`
	SSID    string `json:"ssid,omitempty"`
	BSSID   string `json:"bssid,omitempty"`
	Freq    string `json:"freq,omitempty"`
	Signal  string `json:"signal,omitempty"`
	Bitrate string `json:"bitrate,omitempty"`
	State   string `json:"state"` // connected, disconnected, unavailable
}

// PortInfo is a listening TCP/UDP socket (from `ss`, best-effort).
type PortInfo struct {
	Proto   string `json:"proto"`
	Address string `json:"address"`
	Process string `json:"process,omitempty"`
}

// FirewallInfo is detect-only; linux-ip never changes firewall state.
type FirewallInfo struct {
	Backend string `json:"backend"` // ufw, firewalld, nftables, iptables, none
	Active  bool   `json:"active"`
	Summary string `json:"summary"`
}

// Snapshot is the full collection result rendered by the UI layer.
type Snapshot struct {
	Hostname      string          `json:"hostname"`
	PrimaryIPv4   string          `json:"primaryIpv4,omitempty"`
	PrimaryIface  string          `json:"primaryIface,omitempty"`
	Gateway4      string          `json:"gateway4,omitempty"`
	Gateway6      string          `json:"gateway6,omitempty"`
	Interfaces    []InterfaceInfo `json:"interfaces"`
	Routes4       []Route         `json:"routes4"`
	Routes6       []Route         `json:"routes6"`
	DNS           DNSInfo         `json:"dns"`
	WiFi          []WiFiInfo      `json:"wifi"`
	Listening     []PortInfo      `json:"listening"`
	Firewall      FirewallInfo    `json:"firewall"`
	PublicIP      string          `json:"publicIp,omitempty"` // only when --public-ip
	PublicGeo     string          `json:"publicGeo,omitempty"`
	CollectedWith []string        `json:"collectedWith"`
}

// hasGlobalUnicast reports whether the interface carries a usable address.
func hasGlobalUnicast(iface net.Interface) bool {
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		if ip.To4() != nil {
			return true
		}
	}
	return false
}
