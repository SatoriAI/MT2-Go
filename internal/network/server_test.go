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

// greeter says hello on open and ignores everything after; lift removes the idle limit.
type greeter struct {
	conn *Conn
	lift bool
}

func (g *greeter) Open() error {
	if g.lift {
		g.conn.SetIdleTimeout(0)
	}
	return g.conn.Send([]byte("hi"))
}

func (g *greeter) Handle(context.Context, protocol.Frame) error { return nil }

func (g *greeter) Close() {}

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

func serve(t *testing.T, ln net.Listener, idle time.Duration, lift bool) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, ln, func(c *Conn) Session { return &greeter{conn: c, lift: lift} }, idle,
			slog.New(slog.DiscardHandler))
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
	stop := serve(t, ln, 0, false)

	dial(t, ln.Addr().String())

	if err := stop(); err != nil {
		t.Fatal(err)
	}
}

func TestASilentConnectionIsClosedAfterTheIdleTimeout(t *testing.T) {
	ln := listen(t)
	stop := serve(t, ln, 50*time.Millisecond, false)
	defer func() { _ = stop() }()
	conn := dial(t, ln.Addr().String())

	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("expected the server to close the connection, got %v", err)
	}
}

func TestASessionCanLiftTheIdleTimeout(t *testing.T) {
	ln := listen(t)
	stop := serve(t, ln, 50*time.Millisecond, true)
	defer func() { _ = stop() }()
	conn := dial(t, ln.Addr().String())
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))

	_, err := conn.Read(make([]byte, 1))

	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected the connection to stay open and quiet, got %v", err)
	}
}

func TestStoppingClosesOpenConnections(t *testing.T) {
	ln := listen(t)
	stop := serve(t, ln, 0, false)
	conn := dial(t, ln.Addr().String())

	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("connection still open after stop: %v", err)
	}
}
