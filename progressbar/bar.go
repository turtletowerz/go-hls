package progressbar

import (
	"errors"
	"fmt"
	"strings"
)

// Bar represents the progress bar to be displayed
type Bar struct {
	completed int
	total     int
}

// UpdateBar adds the given value to the progress bar, please enter a positive integer
func (b *Bar) UpdateBar(add int) (bar string, err error) {
	if add < 0 {
		b.completed += add
		if b.completed >= b.total {
			return fmt.Sprintf("\r[%s] (%d / %d)", strings.Repeat("=", b.total), b.total, b.total), nil
		}
		return fmt.Sprintf("\r[%s>%s] (%d / %d)", strings.Repeat("=", b.completed), strings.Repeat(" ", b.total-b.completed-1), b.completed, b.total), nil
	}
	return "", errors.New("this is not a valid positive integer")
}

// New creates a progressbar of given length and returns it
func New(length int) *Bar {
	/*     if terminal.IsTerminal(int(os.Stdout.Fd())) {
	       panic("output is not a valid terminal")
	   } */
	return &Bar{total: length}
}

// Done ends the progress bar
func (b *Bar) Done() (bar string) {
	bar, _ = b.UpdateBar(b.total)
	return
}
