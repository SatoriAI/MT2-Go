// Package handshake implements the time sync every connection starts with. The server
// sends its clock; the client adopts it and echoes it back; the server accepts once the
// echo arrives within a small window, and otherwise resends with a corrected delta.
package handshake

import (
	"errors"
	"math/rand/v2"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// maxLag is how late, in milliseconds, the echo may arrive and still count as in sync.
const maxLag = 500

// ErrForeignID means the reply does not answer the handshake this server sent.
var ErrForeignID = errors.New("handshake: reply carries another id")

// Clock is the server time the handshake shares: milliseconds since the server started,
// as the u32 the client stores. It wraps after 49 days, and so does the client's copy.
type Clock struct{ start time.Time }

func NewClock() Clock { return Clock{start: time.Now()} }

func (c Clock) Now() uint32 {
	return uint32(time.Since(c.start).Milliseconds()) //nolint:gosec // wraps like the client's copy
}

// Sync is the handshake state of one connection.
type Sync struct {
	clock Clock
	sent  protocol.Handshake
}

func NewSync(clock Clock) *Sync {
	return &Sync{clock: clock}
}

// Start returns the first handshake to send.
func (s *Sync) Start() protocol.Handshake {
	s.sent = protocol.Handshake{ID: newID(), Time: s.clock.Now()}
	return s.sent
}

// Answer checks the client's echo. It returns done once the clocks agree; otherwise it
// returns the corrected handshake to send next.
func (s *Sync) Answer(reply protocol.Handshake) (done bool, next protocol.Handshake, err error) {
	if reply.ID != s.sent.ID {
		return false, protocol.Handshake{}, ErrForeignID
	}
	now := s.clock.Now()
	lag := int64(now) - (int64(reply.Time) + int64(reply.Delta))
	if lag >= 0 && lag <= maxLag {
		return true, protocol.Handshake{}, nil
	}
	delta := (int64(now) - int64(reply.Time)) / 2
	if delta < 0 {
		delta = (int64(now) - int64(s.sent.Time)) / 2
	}
	// Half a lag measured in milliseconds; overflowing int32 would take 24 days of lag.
	s.sent = protocol.Handshake{ID: s.sent.ID, Time: now, Delta: int32(delta)} //nolint:gosec // see above
	return false, s.sent, nil
}

// Greeting is what every port sends first: the handshake phase, then the handshake.
func (s *Sync) Greeting() [][]byte {
	return [][]byte{protocol.EncodePhase(protocol.PhaseHandshake), protocol.EncodeHandshake(s.Start())}
}

// Reply decodes the client's echo and answers it: done once in sync, otherwise the
// packet to resend.
func (s *Sync) Reply(body []byte) (done bool, resend []byte, err error) {
	echo, err := protocol.DecodeHandshake(body)
	if err != nil {
		return false, nil, err
	}
	done, next, err := s.Answer(echo)
	if err != nil || done {
		return done, nil, err
	}
	return false, protocol.EncodeHandshake(next), nil
}

// newID picks a handshake id with no 0xD2 byte. The login screen's state checker reads
// its socket a byte at a time looking for 0xD2, the channel-status header, and the
// handshake arrives on that socket first.
func newID() uint32 {
	for {
		id := rand.Uint32()
		if id != 0 && !hasByte(id, byte(protocol.ServerChannelStatus)) {
			return id
		}
	}
}

func hasByte(v uint32, b byte) bool {
	for range 4 {
		if byte(v) == b {
			return true
		}
		v >>= 8
	}
	return false
}
