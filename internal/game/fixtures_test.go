package game

// The fixtures the game's tests share: a world, a connection to read what it is sent, and
// players in it.

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// watcher opens a connection and returns the server's side, to put a player on, and the
// client's, to read what that player is told.
func watcher(t *testing.T) (*network.Conn, net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan *network.Conn, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- network.Serve(ctx, ln, func(c *network.Conn) network.Session {
			accepted <- c
			return idle{}
		}, network.Timeouts{}, slog.New(slog.DiscardHandler))
	}()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	return <-accepted, client
}

// worldWith is a game service whose world holds the mobs of spawns, put there at noon with
// a fixed seed, so every stroll is the same each run.
func worldWith(t *testing.T, spawns ...world.Spawn) *Service {
	t.Helper()
	s := NewService(nil, nil, nil, nil, nil, nil, handshake.NewClock(), netip.AddrPort{}, spawns)
	s.rand = rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a fixed seed, so each run strolls the same
	if err := s.spawn(noon); err != nil {
		t.Fatal(err)
	}
	return s
}

func read(t *testing.T, c net.Conn, n int) []byte {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	b := make([]byte, n)
	if _, err := io.ReadFull(c, b); err != nil {
		t.Fatalf("reading %d bytes: %v", n, err)
	}
	return b
}

// expectNothing checks nothing more arrives for a moment.
func expectNothing(t *testing.T, c net.Conn) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if n, err := c.Read(make([]byte, 1)); n > 0 || err == nil {
		t.Fatal("something arrived")
	}
}

// aPlayer puts a player with a character in the world at x east of the monsters' home,
// alone in view of no one, and returns their client.
func aPlayer(t *testing.T, s *Service, vid, id uint32, name string, x int32) (net.Conn, *present) {
	t.Helper()
	conn, client := watcher(t)
	p := &present{out: conn, char: &account.Character{ID: id, Name: name, Class: account.Sura, HP: 1},
		info:   protocol.CharacterInfo{VID: vid, Name: name},
		add:    protocol.CharacterAdd{VID: vid, X: homeX + x, Y: homeY, Type: protocol.EntityPlayer, MoveSpeed: 150},
		tether: tether{x: homeX + x, y: homeY}}
	s.presence.enter(p)
	return client, p
}

// drain reads whatever arrives until a moment passes with nothing.
func drain(t *testing.T, c net.Conn) []byte {
	t.Helper()
	var got []byte
	b := make([]byte, 4096)
	for {
		_ = c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := c.Read(b)
		got = append(got, b[:n]...)
		if err != nil {
			return got
		}
	}
}

// playingAs is a session's character for the player p, sharing the record's character as
// entering the world does, so a test sees what the world and the session share.
func playingAs(p *present) *character {
	c := &character{vid: p.add.VID, Character: *p.char}
	p.char = &c.Character
	return c
}

// fixedSizes are the lengths of the packets the world sends that have no length of their
// own, taken from their encoders, so they follow the protocol.
var fixedSizes = map[protocol.ServerHeader]int{
	protocol.ServerCharacterAdd:     len(protocol.EncodeCharacterAdd(protocol.CharacterAdd{}, 0)),
	protocol.ServerCharacterInfo:    len(protocol.EncodeCharacterInfo(protocol.CharacterInfo{})),
	protocol.ServerCharacterDelete:  len(protocol.EncodeCharacterDelete(0)),
	protocol.ServerCharacterUpdate:  len(protocol.EncodeCharacterUpdate(protocol.CharacterInfo{}, protocol.CharacterAdd{}, 0)),
	protocol.ServerMove:             len(protocol.EncodeMove(0, protocol.Move{})),
	protocol.ServerPointChange:      len(protocol.EncodePointChange(0, 0, 0, 0)),
	protocol.ServerTarget:           len(protocol.EncodeTarget(0, 0)),
	protocol.ServerDamage:           len(protocol.EncodeDamage(0, 0, 0)),
	protocol.ServerDead:             len(protocol.EncodeDead(0)),
	protocol.ServerItemSet:          len(protocol.EncodeItemSet(protocol.ItemPos{}, 0, 0)),
	protocol.ServerGroundAdd:        len(protocol.EncodeGroundAdd(0, 0, 0, 0)),
	protocol.ServerGroundDel:        len(protocol.EncodeGroundDel(0)),
	protocol.ServerItemOwner:        len(protocol.EncodeItemOwner(0, "")),
	protocol.ServerSpecialEffect:    len(protocol.EncodeSpecialEffect(0, 0)),
	protocol.ServerQuickslotAdd:     len(protocol.EncodeQuickslotAdd(protocol.QuickslotAdd{})),
	protocol.ServerQuickslotDel:     len(protocol.EncodeQuickslotDel(0)),
	protocol.ServerQuickslotSwap:    len(protocol.EncodeQuickslotSwap(0, 0)),
	protocol.ServerSkillLevels:      len(protocol.EncodeSkillLevels(protocol.SkillLevels{})),
	protocol.ServerChangeSkillGroup: len(protocol.EncodeChangeSkillGroup(0)),
	protocol.ServerFlyTarget:        len(protocol.EncodeFlyTarget(protocol.ServerFlyTarget, 0, protocol.FlyTarget{})),
	protocol.ServerAddFlyTarget:     len(protocol.EncodeFlyTarget(protocol.ServerAddFlyTarget, 0, protocol.FlyTarget{})),
	protocol.ServerRefineInfo:       len(protocol.EncodeRefineInfo(protocol.RefineInfo{})),
	protocol.ServerPhase:            len(protocol.EncodePhase(0)),
	protocol.ServerPoints:           len(protocol.EncodePoints(protocol.CharacterPoints{})),
	protocol.ServerMainCharacter:    len(protocol.EncodeMainCharacter(protocol.MainCharacter{})),
	protocol.ServerTime:             len(protocol.EncodeTime(0)),
	protocol.ServerChannel:          len(protocol.EncodeChannel(0)),
}

// packets cuts what a client was sent into whole packets, by each one's length: its own,
// for those that carry it after their header, or its encoder's. A header it does not know
// fails the test, so a check never reads a byte inside another packet as a header.
func packets(t *testing.T, b []byte) [][]byte {
	t.Helper()
	var cut [][]byte
	for len(b) > 0 {
		h := protocol.ServerHeader(b[0])
		n, fixed := fixedSizes[h]
		switch {
		case fixed:
		case (h == protocol.ServerChat || h == protocol.ServerWhisper || h == protocol.ServerScript ||
			h == protocol.ServerShop || h == protocol.ServerSyncPosition) && len(b) >= 3:
			n = int(binary.LittleEndian.Uint16(b[1:3]))
		default:
			t.Fatalf("a packet of unknown header %02x in %x", byte(h), b)
		}
		if n < 1 || n > len(b) {
			t.Fatalf("a packet %02x of %d bytes in %d left: %x", byte(h), n, len(b), b)
		}
		cut, b = append(cut, b[:n]), b[n:]
	}
	return cut
}

// sent is whether what a client was sent holds a packet of the header.
func sent(t *testing.T, b []byte, h protocol.ServerHeader) bool {
	t.Helper()
	return slices.ContainsFunc(packets(t, b), func(p []byte) bool { return protocol.ServerHeader(p[0]) == h })
}
