package game

import "time"

// periodic is a list of things the world loop takes a turn of on a fixed beat, such as a
// poison's damage or a toggle's mana, until each ends. Only what is listed is walked, so an
// idle tick costs nothing more. The zero value is an empty list.
type periodic[T any] []turn[T]

// turn is one listed thing, and when its next turn falls.
type turn[T any] struct {
	item T
	next time.Time
}

// add lists one, its first turn due at next.
func (q *periodic[T]) add(item T, next time.Time) {
	*q = append(*q, turn[T]{item, next})
}

// run steps each one whose turn is due. One whose step returns true is due again every
// after now; the rest are dropped.
func (q *periodic[T]) run(now time.Time, every time.Duration, step func(T) bool) {
	kept := (*q)[:0]
	for _, t := range *q {
		switch {
		case now.Before(t.next):
			kept = append(kept, t)
		case step(t.item):
			kept = append(kept, turn[T]{t.item, now.Add(every)})
		}
	}
	clear((*q)[len(kept):])
	*q = kept
}
