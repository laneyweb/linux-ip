// Package netinfo — lightweight per-interface byte counters for live views.
//
// Reads /sys/class/net/<iface>/statistics/{rx,tx}_bytes directly: no
// subprocess, no root, negligible cost, so the interactive dashboard can
// sample every refresh tick and render btop-style throughput rates.
package netinfo

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// IfaceStats holds cumulative byte counters for one interface.
type IfaceStats struct {
	RxBytes uint64
	TxBytes uint64
}

// SampleIfaceStats snapshots counters for all interfaces.
// Missing files (tunnels, VPNs) yield zero values, never errors.
func SampleIfaceStats() map[string]IfaceStats {
	out := map[string]IfaceStats{}
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		out[name] = IfaceStats{
			RxBytes: readUint(filepath.Join("/sys/class/net", name, "statistics", "rx_bytes")),
			TxBytes: readUint(filepath.Join("/sys/class/net", name, "statistics", "tx_bytes")),
		}
	}
	return out
}

// readUint parses one sysfs counter file, returning 0 on any error.
func readUint(path string) uint64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0
	}
	return v
}
