// Package ui — interactive btop-style dashboard (`linux-ip --interactive`).
//
// Full-screen live view: frozen header (primary/gateway/DNS), scrollable
// body, 2s auto-refresh with per-interface throughput rates from sysfs.
// Any highlighted value can be copied: click it with the mouse, or move
// with ↑/↓ (j/k) and press Enter/c. `a` toggles advanced, `p` fetches the
// public IP on demand (explicit opt-in), `q` quits.
package ui

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/laneyweb/linux-ip/internal/clipboard"
	"github.com/laneyweb/linux-ip/internal/netinfo"
)

// refreshInterval balances liveness with subprocess cost (`ss`, `tailscale`).
const refreshInterval = 2 * time.Second

// InteractiveOptions mirrors the CLI flags relevant to the live view.
type InteractiveOptions struct {
	Advanced bool
	ShowIPv6 bool
	PublicIP bool // fetch public IP once at startup (async, non-blocking)
}

// contentLine is one body row; copy != "" means click/Enter copies it.
// Rows keep unstyled parts (label/shown/extra) so the cursor/hover
// highlight can repaint them on one solid background; text holds the
// normal styled rendering. Sections and static lines use text only.
type contentLine struct {
	text  string
	label string
	shown string // display value (shown column; copy holds the raw value)
	extra string
	copy  string
	row   bool
}

// tickMsg triggers a re-collect; publicIPMsg delivers the async lookup.
type tickMsg time.Time
type publicIPMsg struct{ ip string }

type imodel struct {
	opts   InteractiveOptions
	theme  Theme
	snap   netinfo.Snapshot
	lines  []contentLine
	cursor int // index into lines (always on a copyable line)
	hover  int // line index under the mouse (-1 = none)
	offset int // scroll offset into lines
	width  int
	height int
	toast  string
	ready  bool
	rates  map[string]string // iface -> "↓1.2MB/s ↑300KB/s"
	prev   map[string]netinfo.IfaceStats
	prevAt time.Time
}

// RunInteractive starts the fullscreen dashboard. It requires a TTY;
// piped/scripted use should stick to the default non-interactive output.
func RunInteractive(o InteractiveOptions) error {
	if !isCharDevice(os.Stdin) || !isCharDevice(os.Stdout) {
		return errors.New("interactive mode needs a terminal (omit --interactive when piping)")
	}
	m := imodel{
		opts:  o,
		theme: NewTheme(true),
		rates: map[string]string{},
		hover: -1,
	}
	m.theme.ShowIPv6 = o.ShowIPv6
	m.refresh()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// tickCmd schedules the next refresh.
func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// fetchPublicIPCmd runs the opt-in lookup without freezing the UI.
func fetchPublicIPCmd() tea.Cmd {
	return func() tea.Msg {
		s := netinfo.Collect(netinfo.Options{PublicIP: true})
		return publicIPMsg{ip: s.PublicIP}
	}
}

// isCharDevice reports whether f is a terminal/character device.
func isCharDevice(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// Init implements tea.Model: starts the refresh ticker and, when requested
// via --public-ip, the one-shot async public IP lookup.
func (m imodel) Init() tea.Cmd {
	if m.opts.PublicIP {
		return tea.Batch(tickCmd(), fetchPublicIPCmd())
	}
	return tickCmd()
}

// refresh re-collects state, recomputes rates, and rebuilds body lines,
// preserving the cursor by copied value when the list shape changes.
func (m *imodel) refresh() {
	keep := ""
	if m.cursor < len(m.lines) {
		keep = m.lines[m.cursor].copy
	}
	m.snap = netinfo.Collect(netinfo.Options{IncludeIPv6: m.opts.ShowIPv6})
	if m.snap.PublicIP == "" && keepPublicIP != "" {
		// Keep async-fetched public IP across refresh ticks.
		m.snap.PublicIP = keepPublicIP
	}
	m.updateRates()
	m.lines = m.buildLines()
	m.hover = -1 // positional hover goes stale across rebuilds
	// Land the cursor on a copyable row (line 0 is a section header).
	m.cursor = 0
	for _, idx := range m.copyables() {
		if idx >= m.cursor {
			m.cursor = idx
			break
		}
	}
	if keep != "" {
		for i, l := range m.lines {
			if l.copy != "" && l.copy == keep {
				m.cursor = i
				break
			}
		}
	}
	// Clamp the viewport but never yank it: a wheel-scrolled user stays
	// where they are across refresh ticks. Keyboard moves re-show the
	// cursor via ensureVisible in moveCursor.
	m.clampOffset()
}

// clampOffset keeps the scroll offset inside the rebuilt content.
func (m *imodel) clampOffset() {
	max := len(m.lines) - m.visibleRows()
	if max < 0 {
		max = 0
	}
	if m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// keepPublicIP caches the async lookup so refreshes don't drop it.
var keepPublicIP string

// updateRates diffs sysfs counters against the previous sample.
func (m *imodel) updateRates() {
	now := time.Now()
	cur := netinfo.SampleIfaceStats()
	if m.prev != nil {
		dt := now.Sub(m.prevAt).Seconds()
		if dt > 0 {
			for name, s := range cur {
				p, ok := m.prev[name]
				if !ok {
					continue
				}
				down := rate(s.RxBytes, p.RxBytes, dt)
				up := rate(s.TxBytes, p.TxBytes, dt)
				if down == "0B/s" && up == "0B/s" {
					m.rates[name] = "idle"
				} else {
					m.rates[name] = fmt.Sprintf("↓%s ↑%s", down, up)
				}
			}
		}
	}
	m.prev, m.prevAt = cur, now
}

// rate formats bytes/sec between two counter samples.
func rate(cur, prev uint64, dt float64) string {
	if cur < prev {
		return "0B/s" // counter wrap/reset
	}
	return fmtBytes(float64(cur-prev) / dt) + "/s"
}

// fmtBytes renders a byte rate in B/KB/MB units.
func fmtBytes(v float64) string {
	switch {
	case v >= 1<<20:
		return fmt.Sprintf("%.1fMB", v/(1<<20))
	case v >= 1<<10:
		return fmt.Sprintf("%.0fKB", v/(1<<10))
	default:
		return fmt.Sprintf("%.0fB", v)
	}
}

// copyables returns line indexes that carry a copy value.
func (m imodel) copyables() []int {
	var out []int
	for i, l := range m.lines {
		if l.copy != "" {
			out = append(out, i)
		}
	}
	return out
}

// moveCursor steps ±1 through copyable lines only.
func (m *imodel) moveCursor(dir int) {
	cp := m.copyables()
	if len(cp) == 0 {
		return
	}
	pos := 0
	for i, idx := range cp {
		if idx == m.cursor {
			pos = i
			break
		}
		if idx > m.cursor && dir > 0 {
			pos = i
			break
		}
		if idx > m.cursor {
			pos = i - 1
			break
		}
		pos = i
	}
	pos = (pos + dir + len(cp)) % len(cp)
	m.cursor = cp[pos]
	m.ensureVisible()
}

// scroll moves the viewport by delta lines, clamped to the content.
func (m *imodel) scroll(delta int) {
	m.offset += delta
	m.clampOffset()
}

// visibleRows is the body height after the frozen header and footer.
func (m imodel) visibleRows() int {
	n := m.height - m.headerHeight() - 2
	if n < 1 {
		return 1
	}
	return n
}

// ensureVisible scrolls the offset so the cursor stays on screen.
func (m *imodel) ensureVisible() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if end := m.offset + m.visibleRows(); m.cursor >= end {
		m.offset = m.cursor - m.visibleRows() + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// headerHeight counts the frozen title + PRIMARY box rows.
func (m imodel) headerHeight() int {
	return strings.Count(m.renderHeader(), "\n")
}

// doCopy copies value and sets the confirmation toast.
func (m *imodel) doCopy(value string) {
	if value == "" {
		return
	}
	if err := clipboard.Copy(value); err != nil {
		m.toast = fmt.Sprintf("%s (clipboard unavailable — select manually)", value)
		return
	}
	m.toast = fmt.Sprintf("copied %s", value)
}

// Update implements tea.Model.
func (m imodel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.width <= 0 { // defensive: some ptys report 0x0
			m.width = 80
		}
		if m.height <= 0 {
			m.height = 24
		}
		if !m.ready {
			m.ready = true
		}
		m.ensureVisible()
	case tickMsg:
		m.toast = "" // expire toast on refresh
		m.refresh()
		return m, tickCmd()
	case publicIPMsg:
		if msg.ip != "" {
			keepPublicIP = msg.ip
			m.snap.PublicIP = msg.ip
			m.lines = m.buildLines()
			m.toast = fmt.Sprintf("public IP %s", msg.ip)
		} else {
			m.toast = "public IP lookup failed"
		}
	case tea.MouseMsg:
		// Wheel scrolls the body without moving the keyboard cursor.
		// (msg.Button is checked directly: IsWheel lives on MouseEvent,
		// which MouseMsg doesn't inherit methods from.)
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scroll(-3)
			return m, nil
		case tea.MouseButtonWheelDown:
			m.scroll(3)
			return m, nil
		}
		// Left-click on a copyable row copies it (btop-style picking).
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if idx := m.lineAt(msg.Y); idx >= 0 && m.lines[idx].copy != "" {
				m.cursor = idx
				m.hover = idx
				m.doCopy(m.lines[idx].copy)
			}
		} else if msg.Action == tea.MouseActionMotion {
			// Hover highlight follows the mouse without moving the cursor,
			// so Enter still copies the keyboard-selected row.
			m.hover = m.lineAt(msg.Y)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.hover = -1 // keyboard takes over; drop the mouse highlight
			m.moveCursor(-1)
		case "down", "j":
			m.hover = -1
			m.moveCursor(1)
		case "enter", "c", " ":
			if m.cursor < len(m.lines) {
				m.doCopy(m.lines[m.cursor].copy)
			}
		case "a":
			m.opts.Advanced = !m.opts.Advanced
			m.lines = m.buildLines()
			m.ensureVisible()
		case "p":
			m.toast = "fetching public IP…"
			return m, fetchPublicIPCmd()
		case "r":
			m.refresh()
		case "g":
			m.offset = 0
			if cp := m.copyables(); len(cp) > 0 {
				m.cursor = cp[0]
			}
		case "G":
			m.offset = len(m.lines)
			if cp := m.copyables(); len(cp) > 0 {
				m.cursor = cp[len(cp)-1]
			}
			m.ensureVisible()
		}
	}
	return m, nil
}

// lineAt maps a terminal Y coordinate to a body line index (-1 if header).
func (m imodel) lineAt(y int) int {
	idx := m.offset + (y - m.headerHeight())
	if idx < 0 || idx >= len(m.lines) {
		return -1
	}
	return idx
}

// renderHeader draws the frozen title + PRIMARY/DNS box.
func (m imodel) renderHeader() string {
	t := m.theme
	primary := m.snap.PrimaryIPv4
	if primary == "" {
		primary = "no global IPv4"
	}
	dns := strings.Join(m.snap.DNS.DisplayServers(), ", ")
	if dns == "" {
		dns = "unknown"
	}
	title := t.Title.Render(fmt.Sprintf("linux-ip ● %s  [live %s]", m.snap.Hostname, modeName(m.opts.Advanced)))
	box := fmt.Sprintf("%s %s  %s %s  %s %s\n%s %s",
		t.Label.Render("PRIMARY"), t.Value.Render(primary),
		t.Label.Render("IFACE"), t.Value.Render(orDash(m.snap.PrimaryIface)),
		t.Label.Render("GATEWAY"), t.Value.Render(orDash(m.snap.Gateway4)),
		t.Label.Render("DNS"), t.Value.Render(dns),
	)
	return title + "\n" + t.Box.Render(box) + "\n"
}

func modeName(advanced bool) string {
	if advanced {
		return "advanced"
	}
	return "basic"
}

// renderSelected repaints a row (marker + unstyled parts) on the solid
// cursor/hover background. Parts are used instead of the styled text
// because inner ANSI resets would punch holes in the background.
func (m imodel) renderSelected(marker string, l contentLine) string {
	s := fmt.Sprintf("%-10s %s", l.label, l.shown)
	if l.extra != "" {
		s += "  " + l.extra
	}
	return m.theme.Sel.Render(marker + s)
}

// View implements tea.Model.
func (m imodel) View() string {
	if !m.ready {
		return "loading…"
	}
	var b strings.Builder
	b.WriteString(m.renderHeader())
	end := m.offset + m.visibleRows()
	if end > len(m.lines) {
		end = len(m.lines)
	}
	for i := m.offset; i < end; i++ {
		l := m.lines[i]
		marker := "  "
		if l.copy != "" {
			marker = "• "
			if i == m.cursor {
				marker = "▸ "
			}
		}
		// Cursor (arrows) and hover (mouse) both repaint the row on one
		// solid background so the copy target stands out btop-style.
		if l.row && l.copy != "" && (i == m.cursor || i == m.hover) {
			b.WriteString(m.renderSelected(marker, l) + "\n")
			continue
		}
		b.WriteString(marker + l.text + "\n")
	}
	// Footer: toast (confirmation) + key hints.
	foot := "↑↓/wheel scroll · click/enter copy · a advanced · p public-ip · r refresh · q quit"
	if m.toast != "" {
		foot = m.toast + "  ·  " + "q quit"
		b.WriteString(m.theme.Warn.Render(foot) + "\n")
	} else {
		b.WriteString(m.theme.Dim.Render(foot) + "\n")
	}
	return b.String()
}

// buildLines assembles the scrollable body: quick-copy rows first, then
// interface detail with live rates, then advanced sections when enabled.
func (m imodel) buildLines() []contentLine {
	t := m.theme
	var out []contentLine
	sec := func(name string) {
		out = append(out, contentLine{text: t.Header.Render("── " + name + " ──")})
	}
	row := func(label, value, extra string) {
		line := fmt.Sprintf("%-10s %s", t.Label.Render(label), t.Value.Render(value))
		if extra != "" {
			line += "  " + t.Dim.Render(extra)
		}
		out = append(out, contentLine{text: line, label: label, shown: value, extra: extra, copy: value, row: true})
	}
	plain := func(s string) { out = append(out, contentLine{text: s}) }

	// Quick-copy rows: the key IPs one click away.
	sec("COPY")
	if m.snap.PrimaryIPv4 != "" {
		row("PRIMARY", m.snap.PrimaryIPv4, m.snap.PrimaryIface)
	}
	if m.snap.Gateway4 != "" {
		row("GATEWAY", m.snap.Gateway4, "")
	}
	for _, d := range m.snap.DNS.DisplayServers() {
		row("DNS", d, "")
	}
	if m.snap.Tailscale.Active {
		row("TAILSCALE", m.snap.Tailscale.SelfIP, m.snap.Tailscale.DNSName)
	}
	if m.snap.PublicIP != "" {
		row("PUBLIC", m.snap.PublicIP, "")
	} else {
		plain(t.Dim.Render("PUBLIC  press p to fetch (opt-in)"))
	}

	// Interfaces with live throughput.
	sec("INTERFACES")
	for _, ii := range m.snap.Interfaces {
		if ii.IsLoopback {
			continue
		}
		v4 := addrsToString(ii.IPv4)
		if v4 == "" {
			v4 = "no IPv4"
		}
		r := m.rates[ii.Name]
		if r == "" {
			r = "…"
		}
		first := ""
		if len(ii.IPv4) > 0 {
			first = ii.IPv4[0].IP
		}
		line := fmt.Sprintf("%-10s %s %-18s %s %s %s",
			t.Value.Render(ii.Name), t.Dim.Render(ii.OperState),
			t.Value.Render(v4), t.Dim.Render(ii.MAC),
			t.Dim.Render(ii.Kind), t.Dim.Render(r))
		extra := strings.Join([]string{ii.OperState, ii.MAC, ii.Kind, r}, " ")
		out = append(out, contentLine{text: line, label: ii.Name, shown: v4, extra: extra, copy: first, row: true})
		// Extra IPv4s on the same iface each get their own copyable row.
		if len(ii.IPv4) > 1 {
			for _, a := range ii.IPv4[1:] {
				l := fmt.Sprintf("%-10s %s/%d", "", a.IP, a.PrefixLen)
				shown := fmt.Sprintf("%s/%d", a.IP, a.PrefixLen)
				out = append(out, contentLine{text: l, shown: shown, copy: a.IP, row: true})
			}
		}
		if m.theme.ShowIPv6 {
			for _, a := range ii.IPv6 {
				l := fmt.Sprintf("%-10s %s/%d %s", "", a.IP, a.PrefixLen, t.Dim.Render("v6"))
				shown := fmt.Sprintf("%s/%d", a.IP, a.PrefixLen)
				out = append(out, contentLine{text: l, shown: shown, extra: "v6", copy: a.IP, row: true})
			}
		}
	}

	if !m.opts.Advanced {
		if !m.theme.ShowIPv6 {
			plain(t.Dim.Render("(IPv6 hidden — restart with --ip6)"))
		}
		return out
	}

	sec("ROUTES IPv4")
	for _, r := range m.snap.Routes4 {
		plain(fmt.Sprintf("  %-18s via %-15s dev %s %s",
			r.Destination, orDash(r.Gateway), orDash(r.Iface), t.Dim.Render(r.Proto)))
	}
	if m.theme.ShowIPv6 {
		sec("ROUTES IPv6")
		for _, r := range m.snap.Routes6 {
			plain(fmt.Sprintf("  %-30s via %s", r.Destination, orDash(r.Gateway)))
		}
	}
	sec("DNS DETAIL")
	plain(fmt.Sprintf("  effective: %s", strings.Join(m.snap.DNS.DisplayServers(), ", ")))
	if m.snap.DNS.Current != "" {
		plain(fmt.Sprintf("  current: %s (default-route link)", m.snap.DNS.Current))
	}
	if m.snap.Tailscale.Active || m.snap.Tailscale.BackendState != "" {
		sec("TAILSCALE")
		plain(fmt.Sprintf("  state=%s self=%s host=%s peers=%d",
			orDash(m.snap.Tailscale.BackendState), orDash(m.snap.Tailscale.SelfIP),
			orDash(m.snap.Tailscale.Hostname), m.snap.Tailscale.PeerCount))
	}
	sec("LISTENING")
	if len(m.snap.Listening) == 0 {
		plain("  (none detected)")
	}
	for _, p := range m.snap.Listening {
		plain(fmt.Sprintf("  %-5s %-22s %s", p.Proto, p.Address, t.Dim.Render(p.Process)))
	}
	sec("FIREWALL")
	plain(fmt.Sprintf("  %s active=%v — %s",
		m.snap.Firewall.Backend, m.snap.Firewall.Active, m.snap.Firewall.Summary))
	return out
}
