package game

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// walking is Hendra at the monsters' home, at move speed 150, with Kira 100 east of her who
// sees her; their clients, Hendra's first, with what entering showed them read.
func walking(t *testing.T) (*session, *present, []net.Conn) {
	t.Helper()
	svc := worldWith(t)
	svc.now = func() time.Time { return noon }
	client, hendra := aPlayer(t, svc, 1000, 7, "Hendra", 0)
	viewer, _ := aPlayer(t, svc, 1001, 8, "Kira", 100)
	drain(t, client)
	drain(t, viewer)
	s := &session{svc: svc, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame, playing: playingAs(hendra)}
	return s, hendra, []net.Conn{client, viewer}
}

// walkTo sends a run's move to x east of the home, at a time.
func walkTo(t *testing.T, s *session, x int32, at time.Duration) {
	t.Helper()
	s.svc.now = func() time.Time { return noon.Add(at) }
	body := []byte{1, 0, 0}                                        // FUNC_MOVE
	body = binary.LittleEndian.AppendUint32(body, uint32(homeX+x)) //nolint:gosec // the bytes as sent
	body = binary.LittleEndian.AppendUint32(body, uint32(homeY))
	body = binary.LittleEndian.AppendUint32(body, 0)
	if err := s.onMove(body); err != nil {
		t.Fatal(err)
	}
}

// A move up to 2500 from where she stands is taken at once: the 1000 ahead a running
// client reports, either way of a tether that may be 1000 ahead, and room for moves
// arriving late.
func TestAMoveWithinTheTetherIsTaken(t *testing.T) {
	s, hendra, c := walking(t)

	walkTo(t, s, 2500, 0)

	if hendra.add.X != homeX+2500 || s.playing.X != homeX+2500 {
		t.Fatalf("at %d, kept %d", hendra.add.X, s.playing.X)
	}
	if got := drain(t, c[0]); sent(t, got, protocol.ServerSyncPosition) {
		t.Fatalf("she was put back: %x", got)
	}
}

// A move further than she can have gone is not taken: she is put back where the server has
// her, by SYNC_POSITION to her own client and a stop there for Kira.
func TestAMoveBeyondTheTetherPutsHerBack(t *testing.T) {
	s, hendra, c := walking(t)

	walkTo(t, s, 2600, 0)

	if hendra.add.X != homeX || s.playing.X != homeX {
		t.Fatalf("at %d, kept %d", hendra.add.X, s.playing.X)
	}
	if got := drain(t, c[0]); !bytes.Contains(got, protocol.EncodeSyncPosition(1000, homeX, homeY)) {
		t.Fatalf("told %x", got)
	}
	if got := drain(t, c[1]); !bytes.Contains(got, protocol.EncodeMove(1000, protocol.Move{Type: protocol.MoveWait, X: homeX, Y: homeY})) {
		t.Fatalf("Kira saw %x", got)
	}
}

// The tether follows her at half as fast again as she runs, 1012 a second at move speed
// 150, from her first move, where she stands as a run starts: a second on, a move 3500 off
// is taken, one 3600 off is not.
func TestTheTetherFollowsAtHerSpeed(t *testing.T) {
	for far, taken := range map[int32]bool{3500: true, 3600: false} {
		s, hendra, _ := walking(t)
		walkTo(t, s, 0, 0)
		walkTo(t, s, far, time.Second)
		if got := hendra.add.X == homeX+far; got != taken {
			t.Errorf("%d off a second on: taken %v", far, got)
		}
	}
}

// Moves cannot be chained past the tether: two of 2400 at once, the second lands 4800 from
// where she stood, and is not taken.
func TestMovesCannotBeChainedPastTheTether(t *testing.T) {
	s, hendra, _ := walking(t)

	walkTo(t, s, 2400, 0)
	walkTo(t, s, 4800, 0)

	if hendra.add.X == homeX+4800 {
		t.Fatal("the second move was taken")
	}
}

// Put somewhere by the server, she walks on from there: after a restart in town 50000
// away, a move beside it is taken.
func TestTheTetherFollowsWhereTheServerPutsHer(t *testing.T) {
	s, hendra, _ := walking(t)
	hendra.char.HP = 0
	if !s.svc.record.revive(1000, true, homeX+50000, homeY, noon) {
		t.Fatal("not revived")
	}

	walkTo(t, s, 51000, 0)

	if hendra.add.X != homeX+51000 {
		t.Fatalf("at %d", hendra.add.X-homeX)
	}
}

// Dashed beside a monster by Fast Attack, she walks on from there: a move beside it is
// taken, 5100 from where she stood.
func TestTheTetherFollowsADash(t *testing.T) {
	s, hendra, _ := walking(t)
	dog := &present{add: protocol.CharacterAdd{VID: 5, X: homeX + 5270, Y: homeY, Type: protocol.EntityMonster}}
	s.svc.record.held(func(l locked) {
		l.dash(hendra, dog, 0)
	})

	walkTo(t, s, 5100, 0)

	if hendra.add.X != homeX+5100 {
		t.Fatalf("at %d", hendra.add.X-homeX)
	}
}

// The dead stay where they fell, whatever their client sends.
func TestTheDeadDoNotWalk(t *testing.T) {
	s, hendra, _ := walking(t)
	hendra.char.HP = 0

	walkTo(t, s, 100, 0)

	if hendra.add.X != homeX || s.playing.X != homeX {
		t.Fatalf("at %d, kept %d", hendra.add.X-homeX, s.playing.X-homeX)
	}
}

// A client that runs east for eight seconds at move speed 100, reporting every 300 ms the
// end of its walk 1000 ahead, then turns back west, is never put back.
func TestARunTurningBackIsNotPutBack(t *testing.T) {
	s, hendra, c := walking(t)
	hendra.add.MoveSpeed = 100
	at, x := time.Duration(0), int32(0)
	walkTo(t, s, 0, 0)
	for range 27 { // east at 450 a second
		at += 300 * time.Millisecond
		x = int32(at.Seconds() * 450)
		walkTo(t, s, x+1000, at)
	}
	turned := x
	for range 10 { // and back west
		at += 300 * time.Millisecond
		x -= 135
		walkTo(t, s, x-1000, at)
	}

	if got := drain(t, c[0]); sent(t, got, protocol.ServerSyncPosition) {
		t.Fatalf("put back after turning at %d: %x", turned, got)
	}
}

// A client that stood still banks no more than 2 s: thirty seconds on, a move 5000 off,
// 2025 and 2500 more, is not taken.
func TestStandingStillBanksNoTeleport(t *testing.T) {
	s, hendra, _ := walking(t)
	walkTo(t, s, 0, 0)

	walkTo(t, s, 5000, 30*time.Second)

	if hendra.add.X == homeX+5000 {
		t.Fatal("taken")
	}
}

// Moves held up two seconds and arriving together share the time they took: one 500 off
// spends half a second of it, so the next, 4500 off, still has the rest, 1518 more of
// following, and lands within the tether.
func TestABurstOfMovesSharesItsTime(t *testing.T) {
	s, hendra, c := walking(t)
	walkTo(t, s, 0, 0)

	walkTo(t, s, 500, 2*time.Second)
	walkTo(t, s, 4500, 2*time.Second)

	if got := drain(t, c[0]); hendra.add.X != homeX+4500 || sent(t, got, protocol.ServerSyncPosition) {
		t.Fatalf("at %d, told %x", hendra.add.X-homeX, got)
	}
}

// Risen where she fell, her tether starts afresh there: the time banked before she died
// does not carry her further.
func TestRisingHereStartsTheTetherAfresh(t *testing.T) {
	s, hendra, _ := walking(t)
	walkTo(t, s, 0, 0)
	hendra.char.HP = 0
	if !s.svc.record.revive(1000, false, 0, 0, noon) {
		t.Fatal("not revived")
	}

	walkTo(t, s, 3000, 10*time.Second)

	if hendra.add.X == homeX+3000 {
		t.Fatal("taken")
	}
}
