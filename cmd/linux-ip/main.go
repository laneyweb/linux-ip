// Command linux-ip — entry point.
//
// Flags:
//
//	-a, --advanced   sysadmin detail view
//	--json           machine-readable output (implies no lipgloss color)
//	--no-color       plain text (also honors NO_COLOR env)
//	--ip6            include IPv6 addresses and routes (default: IPv4 only)
//	--public-ip      OPT-IN external IP lookup via api.ipify.org
//	--copy <field>   copy one value to clipboard (ip4|ip6|gateway|dns|public|mac|tailscale)
//	-v, --version    print version
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"

	"github.com/laneyweb/linux-ip/internal/clipboard"
	"github.com/laneyweb/linux-ip/internal/netinfo"
	"github.com/laneyweb/linux-ip/internal/ui"
)

// version is injected at build time: -ldflags "-X main.version=v0.1.0".
var version = "dev"

func main() {
	advanced := flag.Bool("advanced", false, "show sysadmin detail view")
	flag.BoolVar(advanced, "a", false, "show sysadmin detail view (shorthand)")
	jsonOut := flag.Bool("json", false, "output JSON instead of dashboard")
	noColor := flag.Bool("no-color", false, "disable colors")
	showIPv6 := flag.Bool("ip6", false, "include IPv6 addresses and routes")
	flag.BoolVar(showIPv6, "ipv6", false, "include IPv6 addresses and routes (alias)")
	publicIP := flag.Bool("public-ip", false, "opt-in public IP lookup (external request)")
	copyField := flag.String("copy", "", "copy field to clipboard: ip4|ip6|gateway|dns|public|mac|tailscale")
	showVer := flag.Bool("version", false, "print version")
	flag.BoolVar(showVer, "v", false, "print version (shorthand)")
	flag.Parse()

	if *showVer {
		fmt.Printf("linux-ip %s\n", version)
		return
	}

	// NO_COLOR env support per https://no-color.org.
	useColor := !*noColor && os.Getenv("NO_COLOR") == ""
	if useColor {
		// Piped output defaults to plain text unless forced.
		useColor = isatty.IsTerminal(os.Stdout.Fd())
	}

	snap := netinfo.Collect(netinfo.Options{PublicIP: *publicIP, IncludeIPv6: *showIPv6})

	// --copy resolves a single field and exits (works in both views).
	if *copyField != "" {
		val, err := resolveField(snap, strings.ToLower(*copyField))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: "+err.Error())
			os.Exit(1)
		}
		if err := clipboard.Copy(val); err != nil {
			// Still print the value so SSH users can copy manually.
			fmt.Println(val)
			fmt.Fprintln(os.Stderr, "clipboard: "+err.Error())
			return
		}
		fmt.Printf("copied %q to clipboard\n", val)
		return
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(snap); err != nil {
			fmt.Fprintln(os.Stderr, "error: "+err.Error())
			os.Exit(1)
		}
		return
	}

	theme := ui.NewTheme(useColor)
	theme.ShowIPv6 = *showIPv6
	if *advanced {
		fmt.Print(ui.RenderAdvanced(snap, theme))
	} else {
		fmt.Print(ui.RenderBasic(snap, theme))
	}
}

// resolveField maps --copy names to snapshot values.
func resolveField(s netinfo.Snapshot, field string) (string, error) {
	switch field {
	case "ip4", "ip", "ipv4":
		if s.PrimaryIPv4 == "" {
			return "", fmt.Errorf("no primary IPv4 (try --advanced)")
		}
		return s.PrimaryIPv4, nil
	case "ip6", "ipv6":
		for _, ii := range s.Interfaces {
			if ii.Name == s.PrimaryIface && len(ii.IPv6) > 0 {
				return ii.IPv6[0].IP, nil
			}
		}
		return "", fmt.Errorf("no IPv6 on primary interface (run with --ip6)")
	case "gateway", "gw":
		if s.Gateway4 == "" {
			return "", fmt.Errorf("no default gateway found")
		}
		return s.Gateway4, nil
	case "dns":
		// Effective uplink servers, not the 127.0.0.53 stub.
		if servers := s.DNS.DisplayServers(); len(servers) > 0 {
			return strings.Join(servers, ","), nil
		}
		return "", fmt.Errorf("no DNS servers found")
	case "tailscale", "ts":
		if !s.Tailscale.Active {
			return "", fmt.Errorf("tailscale not active")
		}
		return s.Tailscale.SelfIP, nil
	case "public", "public-ip":
		if s.PublicIP == "" {
			return "", fmt.Errorf("no public IP (run with --public-ip)")
		}
		return s.PublicIP, nil
	case "mac":
		for _, ii := range s.Interfaces {
			if ii.Name == s.PrimaryIface {
				return ii.MAC, nil
			}
		}
		return "", fmt.Errorf("no primary interface MAC")
	default:
		return "", fmt.Errorf("unknown field %q (use ip4|ip6|gateway|dns|public|mac|tailscale)", field)
	}
}
