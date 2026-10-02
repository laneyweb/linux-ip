// Package clipboard copies short strings without hard dependencies.
//
// Order: wl-copy (Wayland) -> xclip / xsel (X11) -> OSC52 escape (works over
// SSH in most modern terminals). Returns a descriptive error when nothing
// is available so the CLI can print a hint instead of failing silently.
package clipboard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aymanbagabas/go-osc52/v2"
)

// Copy writes text to the system clipboard using the first available backend.
func Copy(text string) error {
	if _, err := exec.LookPath("wl-copy"); err == nil {
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		cmd := exec.Command("xsel", "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	// OSC52 fallback: works over SSH, no helper binary needed.
	if isTTY() {
		_, err := fmt.Fprint(os.Stdout, osc52.New(text).String())
		return err
	}
	return errors.New("no clipboard backend found (install wl-copy or xclip/xsel)")
}
