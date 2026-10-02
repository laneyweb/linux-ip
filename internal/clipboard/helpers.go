package clipboard

import (
	"os"

	"github.com/mattn/go-isatty"
)

// isTTY reports whether stdout is a terminal (OSC52 only makes sense there).
func isTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}
