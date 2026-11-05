// Package event holds the game's scheduled events: spans of time in which kills give more
// experience or yang, such as Christmas at twice the usual.
package event

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // so the zone loads on a machine without a zone database
	"unicode/utf8"
)

// Zone is the time events are written and announced in: the players' own, Polish time,
// whatever zone the server or the command line's machine is set to. It is read-only.
var Zone = mustLoad("Europe/Warsaw")

func mustLoad(name string) *time.Location {
	z, err := time.LoadLocation(name)
	if err != nil {
		panic(err) // the zone is embedded, so this cannot fail
	}
	return z
}

// Event is a span of time in which every kill's experience, the yang monsters drop, or
// both, are multiplied.
type Event struct {
	ID   int64
	Name string
	// The multipliers, in percent: 200 doubles, 100 leaves it as it is.
	ExpPercent, YangPercent uint32
	Start, End              time.Time // End is the first moment the event no longer runs
}

// The bounds of an event, which the events table checks too.
const (
	MaxNameLength = 64
	MinPercent    = 100 // unchanged; an event must raise one of its multipliers above it
	MaxPercent    = 1000
)

// ErrOverlap means an event's time overlaps another's: one event runs at a time, so a
// player always knows which multiplier they are getting.
var ErrOverlap = errors.New("event: overlaps another event")

// Exp is n experience multiplied by the event, rounded down.
func (e Event) Exp(n uint64) uint64 { return n * uint64(e.ExpPercent) / 100 }

// Yang is n yang multiplied by the event, rounded down.
func (e Event) Yang(n uint32) uint32 { return uint32(uint64(n) * uint64(e.YangPercent) / 100) } //nolint:gosec // at most ten times a drop

// Check is whether the event can be stored: a name of 1 to 64 characters, multipliers of
// 1 to 10 with at least one above 1, and an end after its start.
func (e Event) Check() error {
	switch {
	case e.Name == "" || utf8.RuneCountInString(e.Name) > MaxNameLength:
		return fmt.Errorf("event: the name must be 1 to %d characters", MaxNameLength)
	case !inBounds(e.ExpPercent) || !inBounds(e.YangPercent):
		return errors.New("event: each multiplier must be 1 to 10")
	case e.ExpPercent == MinPercent && e.YangPercent == MinPercent:
		return errors.New("event: it must multiply experience or yang")
	case !e.End.After(e.Start):
		return errors.New("event: the end must come after the start")
	}
	return nil
}

// multiplier is a multiplier as a person writes it: a whole number, or one with up to two
// decimals after a point or a comma.
var multiplier = regexp.MustCompile(`^([0-9]{1,2})(?:[.,]([0-9]{1,2}))?$`)

// ParseMultiplier reads a multiplier, "2" or "1.5" or "1,5", into percent.
func ParseMultiplier(s string) (uint32, error) {
	m := multiplier.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("event: the multiplier %q is not a number with at most two decimals", s)
	}
	whole, _ := strconv.Atoi(m[1])
	cents, _ := strconv.Atoi((m[2] + "00")[:2]) // "5" is 50 hundredths; "" is none
	p := uint32(whole*100 + cents)              //nolint:gosec // at most 9999
	if !inBounds(p) {
		return 0, fmt.Errorf("event: the multiplier %q is not 1 to 10", s)
	}
	return p, nil
}

// Multiplier writes percent as a multiplier with a decimal comma, as Polish players read
// it: 200 is "2", 150 is "1,5".
func Multiplier(percent uint32) string {
	return strings.Replace(strconv.FormatFloat(float64(percent)/100, 'f', -1, 64), ".", ",", 1)
}

func inBounds(percent uint32) bool { return percent >= MinPercent && percent <= MaxPercent }
