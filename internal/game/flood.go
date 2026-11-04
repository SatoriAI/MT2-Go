package game

import "time"

// A player may say floodLines lines, chat and whispers together, in any floodWindow.
// open-mt2's numbers: wide enough that nobody types through them, narrow enough to stop
// the spam tools that send dozens a second.
const (
	floodLines  = 10
	floodWindow = 5 * time.Second
)

// floodLimit remembers when the player's last floodLines lines were let through. A line
// refused is not counted, so a flood does not keep its own block going.
type floodLimit struct {
	sent [floodLines]time.Time
	next int // the oldest of sent, and the slot the next line takes
}

func (f *floodLimit) allow(now time.Time) bool {
	if oldest := f.sent[f.next]; !oldest.IsZero() && now.Sub(oldest) < floodWindow {
		return false
	}
	f.sent[f.next] = now
	f.next = (f.next + 1) % floodLines
	return true
}
