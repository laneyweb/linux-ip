// Package netinfo — interface enumeration via stdlib + sysfs.
//
// Reads /sys/class/net/<iface>/{operstate,speed,duplex,wireless} because those
// paths are stable on both Ubuntu (netplan/systemd-networkd/NM) and Fedora 44
// (NetworkManager). Anything missing is reported as unknown.
package netinfo

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// collectInterfaces enumerates interfaces with addresses and sysfs attributes.
func collectInterfaces() []InterfaceInfo {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]InterfaceInfo, 0, len(ifaces))
	for _, ni := range ifaces {
		info := InterfaceInfo{
			Name:       ni.Name,
			MAC:        ni.HardwareAddr.String(),
			MTU:        ni.MTU,
			OperState:  sysfsString(ni.Name, "operstate", "unknown"),
			Kind:       classify(ni),
			IsLoopback: ni.Flags&net.FlagLoopback != 0,
			IsUp:       ni.Flags&net.FlagUp != 0,
			IsWireless: isWireless(ni.Name),
			IsVirtual:  isVirtual(ni.Name),
		}
		if info.MAC == "" {
			info.MAC = "—"
		}
		// distro: /sys speed may be -1, missing (WiFi), or require root.
		if s := sysfsString(ni.Name, "speed", ""); s != "" {
			if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				info.SpeedMbps = v
			}
		}
		if d := sysfsString(ni.Name, "duplex", ""); d != "" {
			info.Duplex = strings.TrimSpace(d)
		}
		if drv := driverOf(ni.Name); drv != "" {
			info.Driver = drv
		}
		// Addresses via stdlib — no `ip addr` dependency.
		if addrs, err := ni.Addrs(); err == nil {
			for _, a := range addrs {
				ipNet, ok := a.(*net.IPNet)
				if !ok {
					continue
				}
				ones, _ := ipNet.Mask.Size()
				entry := Addr{IP: ipNet.IP.String(), PrefixLen: ones}
				if ipNet.IP.IsLoopback() {
					entry.Scope = "host"
				} else if ipNet.IP.IsLinkLocalUnicast() {
					entry.Scope = "link"
				} else {
					entry.Scope = "global"
				}
				if ipNet.IP.To4() != nil {
					info.IPv4 = append(info.IPv4, entry)
				} else {
					info.IPv6 = append(info.IPv6, entry)
				}
			}
		}
		out = append(out, info)
	}
	return out
}

// sysfsString reads one /sys/class/net file, returning fallback on error.
func sysfsString(iface, file, fallback string) string {
	b, err := os.ReadFile(filepath.Join("/sys/class/net", iface, file))
	if err != nil {
		return fallback
	}
	return strings.TrimSpace(string(b))
}

// isWireless checks for the wireless sysfs marker (works without `iw`).
func isWireless(iface string) bool {
	if _, err := os.Stat(filepath.Join("/sys/class/net", iface, "wireless")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join("/proc/net/wireless")); err == nil {
		if b, err := os.ReadFile("/proc/net/wireless"); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), iface+":") {
					return true
				}
			}
		}
	}
	return false
}

// isVirtual matches common virtual/VPN/container interface prefixes.
func isVirtual(name string) bool {
	for _, p := range []string{"docker", "veth", "br-", "virbr", "tun", "tap", "wg", "tailscale", "zt", "ppp"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// classify maps flags/names to a stable kind label for the UI.
func classify(ni net.Interface) string {
	if ni.Flags&net.FlagLoopback != 0 {
		return "loopback"
	}
	if isWireless(ni.Name) {
		return "wifi"
	}
	if isVirtual(ni.Name) {
		return "virtual"
	}
	if ni.Flags&net.FlagPointToPoint != 0 {
		return "tunnel"
	}
	return "ethernet"
}

// driverOf resolves the /sys device driver symlink (best-effort).
func driverOf(iface string) string {
	link, err := os.Readlink(filepath.Join("/sys/class/net", iface, "device/driver"))
	if err != nil {
		return ""
	}
	return filepath.Base(link)
}
