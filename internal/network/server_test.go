package network

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
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

// flooder opens by sending more than any socket buffer holds, and reports its Send's
// error once a reader that never reads has made it give up.
type flooder struct {
	conn *Conn
	sent chan error
}

func (f *flooder) Open() error {
	err := f.conn.Send(make([]byte, 64<<20))
	f.sent <- err
	return err
}

func (f *flooder) Handle(context.Context, protocol.Frame) error { return nil }

func (f *flooder) Close() {}

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

func TestASendToAClientThatStopsReadingGivesUp(t *testing.T) {
	ln := listen(t)
	sent := make(chan error, 1)
	stop := serve(t, ln, Timeouts{Write: 100 * time.Millisecond}, func(c *Conn) Session {
		return &flooder{conn: c, sent: sent}
	})
	defer func() { _ = stop() }()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	select {
	case err := <-sent:
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Fatalf("expected the send to time out, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the send is still blocked")
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
