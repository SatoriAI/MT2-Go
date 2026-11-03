package game

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// idle is a session that does nothing, so a test can send on its connection directly.
type idle struct{}

func (idle) Open() error                                  { return nil }
func (idle) Handle(context.Context, protocol.Frame) error { return nil }
func (idle) Close()                                       {}

// conns opens n connections and returns the server's side of each. Their clients never
// read.
func conns(t *testing.T, n int) []*network.Conn {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan *network.Conn, n)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- network.Serve(ctx, ln, func(c *network.Conn) network.Session {
			accepted <- c
			return idle{}
		}, network.Timeouts{}, slog.New(slog.DiscardHandler))
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	var server []*network.Conn
	for range n {
		client, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		server = append(server, <-accepted)
	}
	return server
}

// A player whose connection closed while entering never joined the world, and leaving
// it must not touch the others.
func TestAPlayerWhoseEntryFailedLeavesWithoutDisturbingTheOthers(t *testing.T) {
	c := conns(t, 2)
	svc := NewService(nil, nil, handshake.NewClock(), netip.AddrPort{})
	svc.presence.enter(&present{conn: c[0], add: protocol.CharacterAdd{VID: 1}})
	c[1].Close()
	s := &session{svc: svc, conn: c[1], phase: protocol.PhaseLoading,
		playing: &character{vid: 2}, claim: &claim{character: 2, done: make(chan struct{})}}

	if err := s.onEnterGame(); err == nil {
		t.Fatal("entering on a closed connection succeeded")
	}
	s.leaveWorld()

	if len(svc.presence.players) != 1 {
		t.Fatalf("%d players in the world, want the other one only", len(svc.presence.players))
	}
}

// Someone entering a busy map is shown everyone in one send. One send each would fill the
// queue of a client that is reading normally, which the server takes for one that stopped.
func TestEnteringABusyMapDoesNotOverflowTheNewcomersQueue(t *testing.T) {
	c := conns(t, 2)
	var w presence
	w.players = map[uint32]*present{}
	for vid := range uint32(20000) {
		w.players[vid+1] = &present{conn: c[0], add: protocol.CharacterAdd{VID: vid + 1}}
	}

	w.enter(&present{conn: c[1], add: protocol.CharacterAdd{VID: 30000}})

	if err := c[1].Send([]byte{0}); err != nil {
		t.Fatalf("the newcomer's connection: %v", err)
	}
}
