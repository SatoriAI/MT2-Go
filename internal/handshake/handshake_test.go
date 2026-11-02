package handshake

import (
	"errors"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// clockAt returns a clock that currently reads ms milliseconds.
func clockAt(ms int) Clock {
	return Clock{start: time.Now().Add(-time.Duration(ms) * time.Millisecond)}
}

func TestAnswerAcceptsAnEchoThatArrivesInTime(t *testing.T) {
	s := NewSync(clockAt(10_000))
	sent := s.start()

	done, _, err := s.answer(protocol.Handshake{ID: sent.ID, Time: sent.Time})

	if err != nil || !done {
		t.Fatalf("done %v err %v", done, err)
	}
}

func TestAnswerResendsWithHalfTheLagWhenTheEchoIsLate(t *testing.T) {
	s := NewSync(clockAt(10_000))
	sent := s.start()

	done, next, err := s.answer(protocol.Handshake{ID: sent.ID, Time: sent.Time - 2_000})

	if err != nil || done {
		t.Fatalf("done %v err %v", done, err)
	}
	if next.ID != sent.ID || next.Delta < 1_000 || next.Delta > 1_100 {
		t.Fatalf("next %+v", next)
	}
}

func TestAnswerRejectsAReplyToAnotherHandshake(t *testing.T) {
	s := NewSync(clockAt(0))
	sent := s.start()

	_, _, err := s.answer(protocol.Handshake{ID: sent.ID + 1})

	if !errors.Is(err, ErrForeignID) {
		t.Fatalf("got %v", err)
	}
}

func TestAClientThatNeverSyncsIsGivenUp(t *testing.T) {
	s := NewSync(clockAt(10_000))
	sent := s.start()

	var err error
	for range maxRounds + 1 {
		_, _, err = s.answer(protocol.Handshake{ID: sent.ID, Time: sent.Time - 2_000})
	}

	if !errors.Is(err, ErrNoSync) {
		t.Fatalf("got %v", err)
	}
}

func TestNewIDNeverContainsTheChannelStatusHeader(t *testing.T) {
	for range 10_000 {
		if hasByte(newID(), 0xD2) {
			t.Fatal("id contains 0xD2")
		}
	}
}
