package game

import (
	"testing"
	"time"
)

var noon = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

// say lets n lines through f, a millisecond apart from at, and reports how many passed.
func say(f *floodLimit, at time.Time, n int) int {
	passed := 0
	for i := range n {
		if f.allow(at.Add(time.Duration(i) * time.Millisecond)) {
			passed++
		}
	}
	return passed
}

func TestTenLinesPassAndTheEleventhInTheWindowDoesNot(t *testing.T) {
	var f floodLimit

	if got := say(&f, noon, 11); got != floodLines {
		t.Fatalf("%d of 11 lines passed, want %d", got, floodLines)
	}
}

// The window rolls: a line passes as soon as the oldest of the last ten is 5 seconds old.
func TestALinePassesOnceTheOldestIsAWindowOld(t *testing.T) {
	var f floodLimit
	say(&f, noon, floodLines)

	if f.allow(noon.Add(floodWindow - time.Millisecond)) {
		t.Fatal("a line passed while the first was still in the window")
	}
	if !f.allow(noon.Add(floodWindow)) {
		t.Fatal("a line was refused once the first had left the window")
	}
}

// Lines refused while flooding do not count, so the player can speak again a window
// after the lines that passed, however long they kept trying.
func TestRefusedLinesDoNotKeepTheBlockGoing(t *testing.T) {
	var f floodLimit
	say(&f, noon, floodLines)
	say(&f, noon.Add(time.Second), 100)

	if got := say(&f, noon.Add(floodWindow+floodLines*time.Millisecond), floodLines); got != floodLines {
		t.Fatalf("%d of %d lines passed a window after the first ten, want all", got, floodLines)
	}
}
