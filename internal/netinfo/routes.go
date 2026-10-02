// Package netinfo — routes, gateways, listening sockets, firewall.
//
// All helpers shell out to `ip`/`ss` when present and degrade to /proc
// parsing otherwise. Fedora 44 minimal installs may lack `ss` (iproute vs
// iproute-tc split); Ubuntu desktop/server both ship `iproute2`.
package netinfo

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// collectRoutes returns v4/v6 routing tables plus default gateways.
func collectRoutes() (v4, v6 []Route, gw4, gw6 string) {
	v4 = runIPRoute("ip", "-4", "route", "show")
	if len(v4) == 0 {
		v4 = procNetRoute("/proc/net/route")
	}
	v6 = runIPRoute("ip", "-6", "route", "show")
	for _, r := range v4 {
		if r.Destination == "default" && gw4 == "" {
			gw4 = r.Gateway
		}
	}
	for _, r := range v6 {
		if r.Destination == "default" && gw6 == "" {
			gw6 = r.Gateway
		}
	}
	return v4, v6, gw4, gw6
}

// runIPRoute parses `ip route` lines into Route structs.
func runIPRoute(args ...string) []Route {
	out, err := exec.Command(args[0], args[1:]...).Output()
	if err != nil {
		return nil
	}
	var routes []Route
	family := "ipv4"
	if contains(args, "-6") {
		family = "ipv6"
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		r := Route{Family: family}
		fields := strings.Fields(line)
		r.Destination = fields[0]
		for i := 0; i < len(fields); i++ {
			switch fields[i] {
			case "via":
				if i+1 < len(fields) {
					r.Gateway = fields[i+1]
				}
			case "dev":
				if i+1 < len(fields) {
					r.Iface = fields[i+1]
				}
			case "proto":
				if i+1 < len(fields) {
					r.Proto = fields[i+1]
				}
			case "metric":
				if i+1 < len(fields) {
					if m, err := strconv.Atoi(fields[i+1]); err == nil {
						r.Metric = m
					}
				}
			}
		}
		routes = append(routes, r)
	}
	return routes
}

// procNetRoute parses /proc/net/route (hex gateway) as a no-`ip` fallback.
func procNetRoute(path string) []Route {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var routes []Route
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		if first { // skip header
			first = false
			continue
		}
		f := strings.Fields(sc.Text())
		if len(f) < 8 {
			continue
		}
		dst := "default"
		if f[1] != "00000000" {
			if ip := hexLEToIP(f[1]); ip != nil {
				dst = ip.String()
			}
		}
		gw := ""
		if g := hexLEToIP(f[2]); g != nil && g.String() != "0.0.0.0" {
			gw = g.String()
		}
		routes = append(routes, Route{Destination: dst, Gateway: gw, Iface: f[0], Family: "ipv4"})
	}
	return routes
}

// hexLEToIP decodes little-endian hex from /proc/net/route.
func hexLEToIP(h string) net.IP {
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return nil
	}
	return net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

// collectListening parses `ss -tlnp` (best-effort, needs no root for listing).
func collectListening() []PortInfo {
	out, err := exec.Command("ss", "-tln").Output()
	if err != nil {
		return nil
	}
	var ports []PortInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "State") {
			continue
		}
		fields := strings.Fields(line)
		// ss columns: State Recv-Q Send-Q Local:Port Peer:Port ...
		if len(fields) < 5 {
			continue
		}
		proto := "tcp"
		addr := fields[4]
		if strings.Contains(addr, ":::") || strings.HasPrefix(addr, "[") {
			proto = "tcp6"
		}
		proc := ""
		if len(fields) > 5 {
			proc = strings.Join(fields[5:], " ")
			proc = trimProcess(proc)
		}
		ports = append(ports, PortInfo{Proto: proto, Address: addr, Process: proc})
	}
	return ports
}

// trimProcess shortens `users:(("sshd",pid=1,fd=3))` to `sshd`.
func trimProcess(s string) string {
	if i := strings.Index(s, `("`); i >= 0 {
		rest := s[i+2:]
		if j := strings.Index(rest, `"`); j >= 0 {
			return rest[:j]
		}
	}
	if len(s) > 40 {
		return s[:40]
	}
	return s
}

// collectFirewall detects (never modifies) the active firewall backend.
func collectFirewall() FirewallInfo {
	// distro: Ubuntu defaults to ufw, Fedora 44 defaults to firewalld.
	if out, err := exec.Command("ufw", "status").Output(); err == nil {
		s := string(out)
		active := strings.Contains(s, "Status: active")
		return FirewallInfo{Backend: "ufw", Active: active, Summary: firstLine(s)}
	}
	if out, err := exec.Command("firewall-cmd", "--state").Output(); err == nil {
		s := strings.TrimSpace(string(out))
		return FirewallInfo{Backend: "firewalld", Active: s == "running", Summary: "firewalld: " + s}
	}
	if _, err := exec.LookPath("nft"); err == nil {
		if out, err := exec.Command("nft", "list", "ruleset").Output(); err == nil {
			n := strings.Count(string(out), "\n")
			return FirewallInfo{Backend: "nftables", Active: n > 5, Summary: "nftables ruleset lines: " + strconv.Itoa(n)}
		}
	}
	if out, err := exec.Command("iptables", "-S").Output(); err == nil {
		n := strings.Count(string(out), "\n")
		return FirewallInfo{Backend: "iptables", Active: n > 3, Summary: "iptables rules: " + strconv.Itoa(n)}
	}
	return FirewallInfo{Backend: "none", Active: false, Summary: "no firewall backend detected"}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
