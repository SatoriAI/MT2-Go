package network

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// greeter says hello on open and ignores everything after; keepOpen lifts the login
// deadline.
type greeter struct {
	conn     *Conn
	keepOpen bool
}

func (g *greeter) Open() error {
	if g.keepOpen {
		g.conn.KeepOpen()
	}
	return g.conn.Send([]byte("hi"))
}

func (g *greeter) Handle(context.Context, protocol.Frame) error { return nil }

func (g *greeter) Close() {}

// flooder opens by sending more than any socket buffer holds, and reports when its
// connection has closed.
type flooder struct {
	conn   *Conn
	closed chan struct{}
}

func (f *flooder) Open() error { return f.conn.Send(make([]byte, 64<<20)) }

func (f *flooder) Handle(context.Context, protocol.Frame) error { return nil }

func (f *flooder) Close() { close(f.closed) }

// hoarder opens by sending, without pause, more than a client that never reads can take,
// and reports the error Send gave up with.
type hoarder struct {
	conn *Conn
	gave chan error
}

func (h *hoarder) Open() error {
	unit := make([]byte, 64<<10)
	for {
		if err := h.conn.Send(unit); err != nil {
			h.gave <- err
			return err
		}
	}
}

func (h *hoarder) Handle(context.Context, protocol.Frame) error { return nil }

func (h *hoarder) Close() {}

// chorus opens by sending from several goroutines at once: each sends its numbered units,
// every byte of a unit being [goroutine, number].
type chorus struct{ conn *Conn }

const (
	voices        = 4
	unitsPerVoice = 50 // together within queueLength, so none is dropped
	unitSize      = 16
)

func (c *chorus) Open() error {
	var wg sync.WaitGroup
	for v := range voices {
		wg.Go(func() {
			for n := range unitsPerVoice {
				half := bytes.Repeat([]byte{byte(v), byte(n)}, unitSize/4) //nolint:gosec // both below voices and unitsPerVoice
				_ = c.conn.Send(half, half)
			}
		})
	}
	wg.Wait()
	return nil
}

func (c *chorus) Handle(context.Context, protocol.Frame) error { return nil }

func (c *chorus) Close() {}

// leaver answers any packet with farewell, a send at a time, and hangs up: more than the
// writer has sent by then.
type leaver struct{ conn *Conn }

var farewell = bytes.Repeat([]byte("bye "), queueLength-1)

func (l *leaver) Open() error { return l.conn.Send([]byte("hi")) }

func (l *leaver) Handle(context.Context, protocol.Frame) error {
	for range queueLength - 1 {
		if err := l.conn.Send([]byte("bye ")); err != nil {
			return err
		}
	}
	return ErrHangUp
}

func (l *leaver) Close() {}

// flakyListener fails its first Accept the way a process out of file descriptors does.
type flakyListener struct {
	net.Listener
	failed bool
}

func (l *flakyListener) Accept() (net.Conn, error) {
	if !l.failed {
		l.failed = true
		return nil, &net.OpError{Op: "accept", Net: "tcp", Err: syscall.EMFILE}
	}
	return l.Listener.Accept()
}

func serve(t *testing.T, ln net.Listener, limits Timeouts, newSession NewSession) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, ln, newSession, limits, slog.New(slog.DiscardHandler))
	}()
	return func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			return errors.New("Serve did not return")
		}
	}
}

func greet(c *Conn) Session { return &greeter{conn: c} }

func greetAndKeepOpen(c *Conn) Session { return &greeter{conn: c, keepOpen: true} }

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

// dial connects and reads the greeting.
func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	greeting := make([]byte, 2)
	if _, err := io.ReadFull(conn, greeting); err != nil || string(greeting) != "hi" {
		t.Fatalf("greeting %q, %v", greeting, err)
	}
	return conn
}

func TestAFailedAcceptDoesNotStopThePort(t *testing.T) {
	ln := &flakyListener{Listener: listen(t)}
	stop := serve(t, ln, Timeouts{}, greet)

	dial(t, ln.Addr().String())

	if err := stop(); err != nil {
		t.Fatal(err)
	}
}

func TestAConnectionIsClosedAtTheLoginDeadline(t *testing.T) {
	ln := listen(t)
	stop := serve(t, ln, Timeouts{Login: 50 * time.Millisecond}, greet)
	defer func() { _ = stop() }()
	conn := dial(t, ln.Addr().String())

	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("expected the server to close the connection, got %v", err)
	}
}

func TestSendingPacketsDoesNotPostponeTheLoginDeadline(t *testing.T) {
	ln := listen(t)
	stop := serve(t, ln, Timeouts{Login: 200 * time.Millisecond}, greet)
	defer func() { _ = stop() }()
	conn := dial(t, ln.Addr().String())

	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if _, err := conn.Write([]byte{byte(protocol.ClientStateChecker)}); err != nil {
			return // closed while still talking, as it should be
		}
	}
	t.Fatal("a connection that kept sending outlived the login deadline")
}

func TestASessionCanKeepItsConnectionOpen(t *testing.T) {
	ln := listen(t)
	stop := serve(t, ln, Timeouts{Login: 50 * time.Millisecond}, greetAndKeepOpen)
	defer func() { _ = stop() }()
	conn := dial(t, ln.Addr().String())
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))

	_, err := conn.Read(make([]byte, 1))

	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected the connection to stay open and quiet, got %v", err)
	}
}

func TestAClientThatStopsReadingIsDisconnected(t *testing.T) {
	ln := listen(t)
	closed := make(chan struct{})
	stop := serve(t, ln, Timeouts{Write: 100 * time.Millisecond}, func(c *Conn) Session {
		return &flooder{conn: c, closed: closed}
	})
	defer func() { _ = stop() }()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the connection is still open")
	}
}

// A client that stops reading and hangs up still has its session closed at once: the
// player leaves the world without waiting for the write timeout on what is queued.
//
// The clock starts once the first byte arrives, so it leaves out the accept and the
// flooder's 64 MiB send, which take a while under -race on a busy machine. Waiting on the
// queue would take the whole write timeout, far longer than the deadline.
func TestAHangUpClosesTheSessionWithoutWaitingForQueuedWrites(t *testing.T) {
	ln := listen(t)
	closed := make(chan struct{})
	stop := serve(t, ln, Timeouts{Write: 30 * time.Second}, func(c *Conn) Session {
		return &flooder{conn: c, closed: closed}
	})
	defer func() { _ = stop() }()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, make([]byte, 1)); err != nil {
		t.Fatalf("the queued writes never started: %v", err)
	}
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("the session waited on the queued writes")
	}
}

// Send never waits for the client, so a sender holding a lock others need is never held
// up; a client too far behind is dropped instead.
func TestSendingToAClientFarBehindDropsItRatherThanWaiting(t *testing.T) {
	ln := listen(t)
	gave := make(chan error, 1)
	stop := serve(t, ln, Timeouts{}, func(c *Conn) Session { return &hoarder{conn: c, gave: gave} })
	defer func() { _ = stop() }()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	select {
	case err := <-gave:
		if !errors.Is(err, errFallenBehind) {
			t.Fatalf("Send gave up with %v, want errFallenBehind", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Send is waiting for the client")
	}
}

// Sends from several goroutines each arrive whole, and each goroutine's in the order it
// made them.
func TestSendsFromManyGoroutinesArriveWholeAndInOrder(t *testing.T) {
	ln := listen(t)
	stop := serve(t, ln, Timeouts{}, func(c *Conn) Session { return &chorus{conn: c} })
	defer func() { _ = stop() }()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	next := make([]int, voices)
	unit := make([]byte, unitSize)
	for range voices * unitsPerVoice {
		if _, err := io.ReadFull(conn, unit); err != nil {
			t.Fatal(err)
		}
		v, n := int(unit[0]), int(unit[1])
		if !bytes.Equal(unit, bytes.Repeat(unit[:2], unitSize/2)) || n != next[v] {
			t.Fatalf("unit % x, want all of goroutine %d's unit %d", unit, v, next[v])
		}
		next[v]++
	}
}

// What a session sent before hanging up is written before the connection closes, so a
// last packet like Logout's reaches the client.
func TestWhatWasSentBeforeAHangUpReachesTheClient(t *testing.T) {
	ln := listen(t)
	stop := serve(t, ln, Timeouts{}, func(c *Conn) Session { return &leaver{conn: c} })
	defer func() { _ = stop() }()
	conn := dial(t, ln.Addr().String())

	_, _ = conn.Write([]byte{byte(protocol.ClientStateChecker)})

	rest, err := io.ReadAll(conn)
	if err != nil || !bytes.Equal(rest, farewell) {
		t.Fatalf("read %d bytes, %v; want the whole farewell (%d), then the end", len(rest), err, len(farewell))
	}
}

func TestStoppingClosesOpenConnections(t *testing.T) {
	ln := listen(t)
	stop := serve(t, ln, Timeouts{}, greet)
	conn := dial(t, ln.Addr().String())

	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("connection still open after stop: %v", err)
	}
}

func TestServeReturnsAndClosesConnectionsWhenItsListenerIsClosed(t *testing.T) {
	ln := listen(t)
	stop := serve(t, ln, Timeouts{}, greet)
	conn := dial(t, ln.Addr().String())

	_ = ln.Close()

	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("connection still open after the listener closed: %v", err)
	}
	if err := stop(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("expected Serve to report the closed listener, got %v", err)
	}
}
