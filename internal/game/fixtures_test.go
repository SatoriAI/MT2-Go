package game

// The fixtures the game's tests share: a world, a connection to read what it is sent, and
// players in it.

import (
	"context"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/netip"
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
