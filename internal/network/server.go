// Package network owns the TCP side: it accepts connections, gives each one a goroutine
// that cuts the byte stream into packets, and hands every packet to that connection's
// Session. It knows nothing about what the packets mean.
package network

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// maxAcceptDelay caps the pause between retries when accepting fails, for example while
// the process is out of file descriptors.
const maxAcceptDelay = time.Second

// Timeouts bound how long a client can hold a connection without earning it.
type Timeouts struct {
	// Login is counted from the accept: a connection still open after it is closed,
	// however busy it was, unless its session called KeepOpen.
	Login time.Duration
	// Write bounds each Send, so a client that stops reading cannot stall its goroutine.
	Write time.Duration
}

// ErrHangUp is what a session returns to end its connection on purpose, without the
// warning an error would log.
var ErrHangUp = errors.New("network: session hung up")

// Session is the per-connection logic of one port.
type Session interface {
	// Open runs once, before the first read; servers speak first in this protocol.
	Open() error
	// Handle runs for each complete packet. An error ends the connection.
	Handle(context.Context, protocol.Frame) error
	// Close runs once, after the connection has ended for any reason.
	Close()
}

// NewSession builds the Session for a freshly accepted connection.
type NewSession func(*Conn) Session

// Serve accepts connections on ln until ctx ends or accepting fails for good, then closes
// every open connection and waits for their goroutines to finish.
func Serve(ctx context.Context, ln net.Listener, newSession NewSession, limits Timeouts, log *slog.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		stopped bool
		conns   = map[*Conn]struct{}{}
	)
	go func() {
		<-ctx.Done()
		_ = ln.Close()
		mu.Lock()
		stopped = true
		for c := range conns {
			c.Close()
		}
		mu.Unlock()
	}()
	// Deferred calls run last first: cancelling closes the connections, then they are awaited.
	defer wg.Wait()
	defer cancel()

	var delay time.Duration
	for {
		raw, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, net.ErrClosed) {
				return err
			}
			// Running out of file descriptors is not fatal: connections close and free them.
			delay = min(max(2*delay, 5*time.Millisecond), maxAcceptDelay)
			log.Warn("accept failed, retrying", "err", err, "in", delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
			}
			continue
		}
		delay = 0
		c := newConn(raw, limits, log)
		mu.Lock()
		if stopped {
			mu.Unlock()
			c.Close()
			continue
		}
		conns[c] = struct{}{}
		mu.Unlock()
		wg.Go(func() {
			defer func() {
				mu.Lock()
				delete(conns, c)
				mu.Unlock()
			}()
			c.serve(ctx, newSession(c))
		})
	}
}

// Conn is one client connection.
type Conn struct {
	raw   net.Conn
	log   *slog.Logger
	once  sync.Once
	write time.Duration
}

func newConn(raw net.Conn, limits Timeouts, log *slog.Logger) *Conn {
	// Setting a deadline fails only on a closed connection, which the first Read reports.
	if limits.Login > 0 {
		_ = raw.SetReadDeadline(time.Now().Add(limits.Login))
	}
	return &Conn{raw: raw, write: limits.Write, log: log.With("remote", raw.RemoteAddr().String())}
}

// Log is this connection's logger, tagged with the remote address.
func (c *Conn) Log() *slog.Logger { return c.log }

// Send writes packets in order. Only the connection's own goroutine sends, so writes
// never interleave.
func (c *Conn) Send(packets ...[]byte) error {
	if c.write > 0 {
		if err := c.raw.SetWriteDeadline(time.Now().Add(c.write)); err != nil {
			return err
		}
	}
	for _, p := range packets {
		if _, err := c.raw.Write(p); err != nil {
			return err
		}
	}
	return nil
}

// KeepOpen lifts the login deadline. A session calls it once the client has proved who it
// is; the player may then stay as long as they like.
func (c *Conn) KeepOpen() { _ = c.raw.SetReadDeadline(time.Time{}) }

// Close ends the connection; the read loop then returns and the Session is closed.
func (c *Conn) Close() {
	c.once.Do(func() { _ = c.raw.Close() })
}

func (c *Conn) serve(ctx context.Context, s Session) {
	defer s.Close()
	defer c.Close()

	if err := s.Open(); err != nil {
		c.log.Warn("open failed", "err", err)
		return
	}
	var buf []byte
	chunk := make([]byte, 4096)
	for {
		n, err := c.raw.Read(chunk)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				c.log.Debug("connection ended", "err", err)
			}
			return
		}
		buf = append(buf, chunk[:n]...)
		if buf, err = c.dispatch(ctx, buf, s); err != nil {
			// Shutting down cuts sessions short; that is not the client's doing.
			if !errors.Is(err, ErrHangUp) && ctx.Err() == nil {
				c.log.Warn("closing connection", "err", err)
			}
			return
		}
	}
}

// dispatch hands every complete packet in buf to the session and returns what is left:
// the start of a packet whose remaining bytes have not arrived yet.
func (c *Conn) dispatch(ctx context.Context, buf []byte, s Session) ([]byte, error) {
	for {
		frame, used, err := protocol.NextFrame(buf)
		if errors.Is(err, protocol.ErrIncomplete) {
			return buf, nil
		}
		if err != nil {
			return nil, err
		}
		if err := s.Handle(ctx, frame); err != nil {
			return nil, err
		}
		buf = buf[used:]
	}
}
