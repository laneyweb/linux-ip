// Package netinfo — Tailscale awareness (best-effort, optional binary).
//
// When `tailscale` is installed and logged in, we surface the Tailnet
// identity (100.x self IP, hostname, peer count) so VPN mesh state is
// visible next to the tailscale0 interface. Absent binary or logged-out
// client yields Active=false and the UI hides the section.
package netinfo

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// tailscaleJSON mirrors the subset of `tailscale status --json` we need.
type tailscaleJSON struct {
	BackendState string `json:"BackendState"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Self         *struct {
		HostName string   `json:"HostName"`
		DNSName  string   `json:"DNSName"`
	} `json:"Self"`
	Peer map[string]struct {
		HostName string `json:"HostName"`
	} `json:"Peer"`
}

// collectTailscale queries `tailscale status --json`, falling back to
// `tailscale ip` when JSON output is unavailable (older clients).
func collectTailscale() TailscaleInfo {
	out, err := exec.Command("tailscale", "status", "--json").Output()
	if err == nil {
		var ts tailscaleJSON
		if jerr := json.Unmarshal(out, &ts); jerr == nil {
			ti := TailscaleInfo{BackendState: ts.BackendState}
			if ts.Self != nil {
				ti.Hostname = ts.Self.HostName
				ti.DNSName = strings.TrimSuffix(ts.Self.DNSName, ".")
			}
			for _, ip := range ts.TailscaleIPs {
				// Prefer the IPv4 (100.x) identity for display.
				if !strings.Contains(ip, ":") {
					ti.SelfIP = ip
					break
				}
			}
			if ti.SelfIP == "" && len(ts.TailscaleIPs) > 0 {
				ti.SelfIP = ts.TailscaleIPs[0]
			}
			ti.PeerCount = len(ts.Peer)
			ti.Active = ts.BackendState == "Running" && ti.SelfIP != ""
			if ti.Active {
				ti.Summary = fmt.Sprintf("%s (%s, %d peers)", ti.SelfIP, ti.DNSName, ti.PeerCount)
			} else {
				ti.Summary = "tailscale " + ts.BackendState
			}
			return ti
		}
	}
	// Fallback: `tailscale ip` prints self IPs, one per line.
	if out, err := exec.Command("tailscale", "ip").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if ip := strings.TrimSpace(line); ip != "" && !strings.Contains(ip, ":") {
				return TailscaleInfo{Active: true, SelfIP: ip, Summary: ip}
			}
		}
	}
	return TailscaleInfo{Active: false}
}
