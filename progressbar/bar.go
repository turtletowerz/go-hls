package progressbar

import (
	"fmt"
	"strings"
)

type Bar struct {
	completed int
	total     int
}

func (b *Bar) updateBar() {
	// TODO: fix this shit because it does some weird stuff
	if b.completed == b.total {
		fmt.Printf("\r[%s] (%d / %d)", strings.Repeat("=", b.total), b.total, b.total)
		fmt.Println()
	} else {
		fmt.Printf("\r[%s>%s] (%d / %d)", strings.Repeat("=", b.completed-1), strings.Repeat(" ", b.total-b.completed), b.completed, b.total)
	}
}

func (b *Bar) Done() {
	b.completed = b.total
	b.updateBar()
}

func (b *Bar) Add(amount int) {
	b.completed = b.completed + amount
	b.updateBar()
}

func New(length int) *Bar {
	//if terminal.IsTerminal(int(os.Stdout.Fd())) {
	//	panic("output is not a valid terminal")
	//}
	return &Bar{total: length}
}
