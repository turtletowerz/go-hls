package progressbar

import (
	"fmt"
	"strings"
)

// Bar represents the progress bar to be displayed
type Bar struct {
	completed int
	total     int
}

// UpdateBar adds the given value to the progress bar
func (b *Bar) UpdateBar(add uint32) (bar string) {
	b.completed += int(add)
	if b.completed >= b.total {
		return fmt.Sprintf("\r[%s] (%d / %d)\n", strings.Repeat("=", b.total), b.total, b.total)
	}
	return fmt.Sprintf("\r[%s>%s] (%d / %d)", strings.Repeat("=", b.completed), strings.Repeat(" ", b.total-b.completed-1), b.completed, b.total)
}

// New creates a progressbar of given length and returns it
func New(length int) *Bar {
	/*     if terminal.IsTerminal(int(os.Stdout.Fd())) {
	       panic("output is not a valid terminal")
	   } */
	return &Bar{total: length}
}
