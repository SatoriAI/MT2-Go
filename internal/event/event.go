// Package event holds the game's scheduled events: spans of time in which kills give more
// experience, such as Christmas at twice the usual.
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

// Event is a span of time in which every kill's experience is multiplied. Only experience
// is, for now: drops and gold become further multipliers once they exist.
type Event struct {
	ID         int64
	Name       string
	ExpPercent uint32    // the multiplier, in percent: 200 doubles
	Start, End time.Time // End is the first moment the event no longer runs
}

// The bounds of an event, which the events table checks too.
const (
	MaxNameLength = 64
	MinPercent    = 101 // an event gives more, or it is no event
	MaxPercent    = 1000
)

// ErrOverlap means an event's time overlaps another's: one event runs at a time, so a
// player always knows which multiplier they are getting.
var ErrOverlap = errors.New("event: overlaps another event")

// Exp is n experience multiplied by the event, rounded down.
func (e Event) Exp(n uint64) uint64 { return n * uint64(e.ExpPercent) / 100 }

// Check is whether the event can be stored: a name of 1 to 64 characters, a multiplier
// above 1 and at most 10, and an end after its start.
func (e Event) Check() error {
	switch {
	case e.Name == "" || utf8.RuneCountInString(e.Name) > MaxNameLength:
		return fmt.Errorf("event: the name must be 1 to %d characters", MaxNameLength)
	case e.ExpPercent < MinPercent || e.ExpPercent > MaxPercent:
		return errors.New("event: the multiplier must be above 1 and at most 10")
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
	p := whole*100 + cents
	if p < MinPercent || p > MaxPercent {
		return 0, fmt.Errorf("event: the multiplier %q is not above 1 and at most 10", s)
	}
	return uint32(p), nil
}

// Multiplier writes percent as a multiplier with a decimal comma, as Polish players read
// it: 200 is "2", 150 is "1,5".
func Multiplier(percent uint32) string {
	return strings.Replace(strconv.FormatFloat(float64(percent)/100, 'f', -1, 64), ".", ",", 1)
}
