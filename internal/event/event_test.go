package event

import (
	"strings"
	"testing"
	"time"
)

func TestAMultiplierIsReadAsAPersonWritesIt(t *testing.T) {
	for in, want := range map[string]uint32{"1": 100, "2": 200, "1.5": 150, "1,5": 150, "1.25": 125, "10": 1000, "1.01": 101, "1,05": 105} {
		if got, err := ParseMultiplier(in); err != nil || got != want {
			t.Errorf("%q: got %d, %v; want %d", in, got, err, want)
		}
	}
}

func TestAMultiplierBelowOneTooLargeOrNotWrittenPlainlyIsRefused(t *testing.T) {
	for _, in := range []string{"0.99", "0.5", "-2", "10.01", "", "two", "NaN", "Inf", "1e1", "0x1p1", "1.006", "1.", " 2"} {
		if got, err := ParseMultiplier(in); err == nil {
			t.Errorf("%q: got %d, want an error", in, got)
		}
	}
}

func TestAMultiplierIsWrittenWithADecimalComma(t *testing.T) {
	for percent, want := range map[uint32]string{200: "2", 150: "1,5", 115: "1,15", 1000: "10"} {
		if got := Multiplier(percent); got != want {
			t.Errorf("%d: got %q, want %q", percent, got, want)
		}
	}
}

func TestAnEventMultipliesExperienceRoundingDown(t *testing.T) {
	e := Event{ExpPercent: 150}
	if got := e.Exp(15); got != 22 {
		t.Fatalf("got %d, want 22", got)
	}
}

func TestAnEventMultipliesYangRoundingDown(t *testing.T) {
	e := Event{YangPercent: 150}
	for n, want := range map[uint32]uint32{15: 22, 101: 151} {
		if got := e.Yang(n); got != want {
			t.Errorf("%d: got %d, want %d", n, got, want)
		}
	}
}

func TestOnlyASensibleEventCanBeStored(t *testing.T) {
	start := time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC)
	good := Event{Name: "Boże Narodzenie", ExpPercent: 200, YangPercent: 100, Start: start, End: start.Add(72 * time.Hour)}
	if err := good.Check(); err != nil {
		t.Fatalf("a good event refused: %v", err)
	}
	yangOnly := good
	yangOnly.ExpPercent, yangOnly.YangPercent = 100, 150
	if err := yangOnly.Check(); err != nil {
		t.Fatalf("an event of yang alone refused: %v", err)
	}
	for name, change := range map[string]func(*Event){
		"no name":          func(e *Event) { e.Name = "" },
		"a name too long":  func(e *Event) { e.Name = strings.Repeat("ż", MaxNameLength+1) },
		"no bonus":         func(e *Event) { e.ExpPercent = 100 },
		"too large":        func(e *Event) { e.ExpPercent = MaxPercent + 1 },
		"too much yang":    func(e *Event) { e.YangPercent = MaxPercent + 1 },
		"less yang":        func(e *Event) { e.YangPercent = MinPercent - 1 },
		"ending at start":  func(e *Event) { e.End = e.Start },
		"ending before it": func(e *Event) { e.End = e.Start.Add(-time.Hour) },
	} {
		e := good
		change(&e)
		if err := e.Check(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	long := good
	long.Name = strings.Repeat("ż", MaxNameLength)
	if err := long.Check(); err != nil {
		t.Errorf("a name of %d characters, each of two bytes, refused: %v", MaxNameLength, err)
	}
}
